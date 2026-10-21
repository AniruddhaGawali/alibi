package admin

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"backend/internal/llm"
	"backend/internal/settings"
	"backend/internal/storage"
	"backend/internal/user"
	"backend/pkg/utils"
)

// ModelChecker lists the models a case may use and confirms a model exists and
// outputs text only; *llm.Clients implements it.
type ModelChecker interface {
	Enabled() bool
	CheckTextModel(ctx context.Context, model string) error
	ListTextModels(ctx context.Context) ([]llm.ModelInfo, error)
}

type modelsResp struct {
	Models  []llm.ModelInfo `json:"models"`
	Default string          `json:"default"` // llm.DefaultModel
}

// ListModels handles GET /api/admin/models: the OpenRouter models a case can be
// assigned (text output only, within the price cap), for the case builder's picker.
func (h *Handler) ListModels(w http.ResponseWriter, r *http.Request) {
	if !h.models.Enabled() {
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "OpenRouter is not configured, so the model list is unavailable", nil)
		return
	}
	models, err := h.models.ListTextModels(r.Context())
	if err != nil {
		utils.LogErrorCtx(r.Context(), "list models failed", "err", err)
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "could not load the model list from OpenRouter", nil)
		return
	}
	utils.SuccessResponse(w, http.StatusOK, modelsResp{Models: models, Default: llm.DefaultModel})
}

// SettingsService serves and saves the global settings; *settings.Service implements it.
type SettingsService interface {
	Current(ctx context.Context) settings.Settings
	Update(ctx context.Context, s settings.Settings, by string) (*settings.Settings, error)
}

type Handler struct {
	store     Store
	models    ModelChecker
	userID    func(context.Context) string // auth.UserID in production
	tokenRole func(context.Context) string // the role claim of the access token; nil skips that check
	images    storage.Provider             // nil: uploads answer 503
	settings  SettingsService              // nil: the config routes answer 503
	tokens    TokenGranter                 // nil: granting tokens answers 503
	limits    LimitResetter                // nil: resetting limits answers 503
}

// LimitResetter clears one player's interrogation limits for today;
// *interrogation.RedisGuard implements it.
type LimitResetter interface {
	ResetLimits(ctx context.Context, userID string) error
}

// WithLimitReset enables POST /api/admin/users/{id}/reset-limits.
func WithLimitReset(l LimitResetter) Option { return func(h *Handler) { h.limits = l } }

// TokenGranter adds question tokens to one player; *wallet.Service implements it.
type TokenGranter interface {
	Grant(ctx context.Context, userID string, amount int64) (int64, error)
	// MaxTokens is the most a player can hold (the grant stops there).
	MaxTokens(ctx context.Context) int64
}

// WithTokens enables POST /api/admin/users/{id}/tokens.
func WithTokens(t TokenGranter) Option { return func(h *Handler) { h.tokens = t } }

// Option configures a Handler.
type Option func(*Handler)

// WithTokenRole makes AdminOnly refuse a token whose role claim is not admin before
// touching the database.
func WithTokenRole(f func(context.Context) string) Option {
	return func(h *Handler) { h.tokenRole = f }
}

// WithImages enables POST /api/admin/upload and the removal of replaced images.
func WithImages(p storage.Provider) Option { return func(h *Handler) { h.images = p } }

// WithSettings enables GET/PUT /api/admin/config.
func WithSettings(s SettingsService) Option { return func(h *Handler) { h.settings = s } }

func NewHandler(store Store, models ModelChecker, userID func(context.Context) string, opts ...Option) *Handler {
	h := &Handler{store: store, models: models, userID: userID}
	for _, o := range opts {
		o(h)
	}
	return h
}

// AdminOnly lets the request through only for admins. The token's role claim is
// checked first (cheap, and enough to turn away players), then users.role in the
// database, so an admin who was demoted loses access at once even though their
// token still says admin. A newly made admin has to log in again to get the claim.
// Everyone else gets 403.
func (h *Handler) AdminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := h.userID(r.Context())
		if h.tokenRole != nil && h.tokenRole(r.Context()) != user.RoleAdmin {
			utils.LogWarnCtx(r.Context(), "admin route denied", "user_id", uid, "path", r.URL.Path, "reason", "token role")
			utils.ErrorResponse(w, http.StatusForbidden, "admin access required", nil)
			return
		}
		role, err := h.store.Role(r.Context(), uid)
		if err != nil && !errors.Is(err, ErrNotFound) {
			utils.LogErrorCtx(r.Context(), "admin check failed", "user_id", uid, "err", err)
			utils.ErrorResponse(w, http.StatusInternalServerError, "could not check permissions", nil)
			return
		}
		if role != user.RoleAdmin {
			utils.LogWarnCtx(r.Context(), "admin route denied", "user_id", uid, "path", r.URL.Path, "reason", "database role")
			utils.ErrorResponse(w, http.StatusForbidden, "admin access required", nil)
			return
		}
		next(w, r)
	}
}

// DefaultPenaltyTrophies is what a wrong accusation costs when an admin does not say; it
// matches the stories.penalty_trophies column default.
const DefaultPenaltyTrophies = 5

type createCaseReq struct {
	Title           string   `json:"title" validate:"required,max=200"`
	Synopsis        string   `json:"synopsis" validate:"required,max=2000"`
	MainPlot        string   `json:"main_plot" validate:"max=10000"`
	ThumbnailURL    string   `json:"thumbnail_url" validate:"omitempty,https_url,max=2048"`
	DifficultyLevel string   `json:"difficulty_level" validate:"required,oneof=easy medium hard"`
	RewardTrophies  int32    `json:"reward_trophies" validate:"min=0,max=1000"`            // paid for a correct accusation
	PenaltyTrophies *int32   `json:"penalty_trophies" validate:"omitempty,min=0,max=1000"` // taken for a wrong one; DefaultPenaltyTrophies when left out
	Resolution      string   `json:"resolution" validate:"max=10000"`                      // reveals the truth once the case is closed
	Hints           []string `json:"hints" validate:"max=3,dive,required,max=1000"`        // progressive hints, easiest first
	ChatTips        []string `json:"chat_tips" validate:"max=5,dive,required,max=300"`     // public tips for investigators
	IsPublished     bool     `json:"is_published"`                                         // false (a draft) when left out
	LLMModel        string   `json:"llm_model" validate:"omitempty,max=100"`               // llm.DefaultModel when left out
	MusicTrack      string   `json:"music_track"`                                          // a MusicTracks key; "" (left out) plays the main theme
}

type updateCaseReq struct {
	Title           *string   `json:"title" validate:"omitempty,min=1,max=200"`
	Synopsis        *string   `json:"synopsis" validate:"omitempty,min=1,max=2000"`
	MainPlot        *string   `json:"main_plot" validate:"omitempty,max=10000"`
	ThumbnailURL    *string   `json:"thumbnail_url" validate:"omitempty,https_url,max=2048"`
	DifficultyLevel *string   `json:"difficulty_level" validate:"omitempty,oneof=easy medium hard"`
	RewardTrophies  *int32    `json:"reward_trophies" validate:"omitempty,min=0,max=1000"`
	PenaltyTrophies *int32    `json:"penalty_trophies" validate:"omitempty,min=0,max=1000"`
	Resolution      *string   `json:"resolution" validate:"omitempty,max=10000"`
	Hints           *[]string `json:"hints" validate:"omitempty,max=3,dive,required,max=1000"`    // [] clears them
	ChatTips        *[]string `json:"chat_tips" validate:"omitempty,max=5,dive,required,max=300"` // [] clears them
	IsPublished     *bool     `json:"is_published"`
	LLMModel        *string   `json:"llm_model" validate:"omitempty,max=100"`
	MusicTrack      *string   `json:"music_track"` // "" switches back to the main theme
}

// ListCases handles GET /api/admin/cases: every case, drafts included.
func (h *Handler) ListCases(w http.ResponseWriter, r *http.Request) {
	cases, err := h.store.ListCases(r.Context())
	if err != nil {
		utils.LogErrorCtx(r.Context(), "list admin cases failed", "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not load the cases", nil)
		return
	}
	utils.SuccessResponse(w, http.StatusOK, cases)
}

// GetCase handles GET /api/admin/cases/{id}: the case with its suspects (secrets
// included) and scenes.
func (h *Handler) GetCase(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	c, err := h.store.GetCase(r.Context(), id)
	if err != nil {
		h.storeFailed(w, r, err, "case", "load case failed")
		return
	}
	utils.SuccessResponse(w, http.StatusOK, c)
}

// CreateCase handles POST /api/admin/cases. A new case is a draft: add suspects,
// then publish it with PATCH.
func (h *Handler) CreateCase(w http.ResponseWriter, r *http.Request) {
	var req createCaseReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	if !checkMusic(w, req.MusicTrack) {
		return
	}
	if req.LLMModel == "" {
		req.LLMModel = llm.DefaultModel
	}
	if !h.checkModel(w, r, req.LLMModel) {
		return
	}
	hints := req.Hints
	if hints == nil {
		hints = []string{}
	}
	penalty := int32(DefaultPenaltyTrophies)
	if req.PenaltyTrophies != nil {
		penalty = *req.PenaltyTrophies
	}
	c, err := h.store.CreateCase(r.Context(), CaseInput{
		Title: req.Title, Synopsis: req.Synopsis, MainPlot: req.MainPlot, ThumbnailURL: req.ThumbnailURL,
		DifficultyLevel: req.DifficultyLevel, RewardTrophies: req.RewardTrophies, PenaltyTrophies: penalty, Resolution: req.Resolution, Hints: hints,
		IsPublished: req.IsPublished, LLMModel: req.LLMModel, MusicTrack: req.MusicTrack, ChatTips: req.ChatTips,
	})
	if err != nil {
		h.storeFailed(w, r, err, "case", "create case failed")
		return
	}
	utils.LogInfoCtx(r.Context(), "case created", "admin_id", h.userID(r.Context()), "case_id", c.ID, "llm_model", c.LLMModel)
	utils.SuccessResponse(w, http.StatusCreated, c)
}

type reorderCasesReq struct {
	CaseIDs []string `json:"case_ids" validate:"required,min=1,max=1000,unique,dive,uuid"`
}

// ReorderCases handles PUT /api/admin/cases/order: case_ids lists every case, first to
// last. The players' dashboard shows the published ones in this order.
func (h *Handler) ReorderCases(w http.ResponseWriter, r *http.Request) {
	var req reorderCasesReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	if err := h.store.ReorderCases(r.Context(), req.CaseIDs); err != nil {
		h.storeFailed(w, r, err, "case order", "reorder cases failed")
		return
	}
	utils.LogInfoCtx(r.Context(), "cases reordered", "admin_id", h.userID(r.Context()), "cases", len(req.CaseIDs))
	utils.SuccessResponse(w, http.StatusOK, map[string][]string{"case_ids": req.CaseIDs})
}

// UpdateCase handles PATCH /api/admin/cases/{id}. Only the fields sent change.
func (h *Handler) UpdateCase(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req updateCaseReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	if req == (updateCaseReq{}) {
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed", map[string]string{"body": "send at least one field to change"})
		return
	}
	if req.MusicTrack != nil && !checkMusic(w, *req.MusicTrack) {
		return
	}
	if req.LLMModel != nil && !h.checkModel(w, r, *req.LLMModel) {
		return
	}
	var hints, tips []string
	if req.Hints != nil {
		hints = append([]string{}, *req.Hints...)
	}
	if req.ChatTips != nil {
		tips = append([]string{}, *req.ChatTips...)
	}
	c, old, err := h.store.UpdateCase(r.Context(), id, CasePatch{
		Title: req.Title, Synopsis: req.Synopsis, MainPlot: req.MainPlot, ThumbnailURL: req.ThumbnailURL,
		DifficultyLevel: req.DifficultyLevel, RewardTrophies: req.RewardTrophies, PenaltyTrophies: req.PenaltyTrophies, Resolution: req.Resolution, Hints: hints,
		IsPublished: req.IsPublished, LLMModel: req.LLMModel, MusicTrack: req.MusicTrack, ChatTips: tips,
	})
	if err != nil {
		h.storeFailed(w, r, err, "case", "update case failed")
		return
	}
	h.dropImage(r.Context(), old, c.ThumbnailURL)
	utils.LogInfoCtx(r.Context(), "case updated", "admin_id", h.userID(r.Context()), "case_id", c.ID, "llm_model", c.LLMModel, "published", c.IsPublished)
	utils.SuccessResponse(w, http.StatusOK, c)
}

// DeleteCase handles DELETE /api/admin/cases/{id}. A case anyone has played or is
// playing is kept (409): unpublish it instead, so their results stay.
func (h *Handler) DeleteCase(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	images, err := h.store.DeleteCase(r.Context(), id)
	if err != nil {
		h.storeFailed(w, r, err, "case", "delete case failed")
		return
	}
	for _, u := range images {
		h.dropImage(r.Context(), u, "")
	}
	utils.LogInfoCtx(r.Context(), "case deleted", "admin_id", h.userID(r.Context()), "case_id", id)
	utils.SuccessResponse(w, http.StatusOK, map[string]string{"id": id})
}

// MusicTracks are the case music keys. Each is a static file in the bucket at
// music/<key>.ogg (uploaded by hand, like the seeded art); the frontend has the same
// list with display names. A case with no track ("") plays music/main.ogg, which is
// also what plays whenever a player is not inside a case.
var MusicTracks = []string{"detective-caution", "bloodmoney", "too-crazy", "solving-the-crime"}

// checkMusic accepts "" or a MusicTracks key, answering 422 itself otherwise.
func checkMusic(w http.ResponseWriter, track string) bool {
	if track == "" || slices.Contains(MusicTracks, track) {
		return true
	}
	utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed",
		map[string]string{"music_track": "must be one of: " + strings.Join(MusicTracks, ", ") + " (or empty for the main theme)"})
	return false
}

// pathID reads a UUID path value, answering 400 itself when it is not one.
func pathID(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	id := r.PathValue(name)
	if errs := utils.Validate(struct {
		ID string `validate:"required,uuid"`
	}{id}); errs != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid "+strings.ReplaceAll(name, "_", " "), map[string]string{name: "must be a valid UUID"})
		return "", false
	}
	return id, true
}

// storeFailed answers a store error: domain errors map to 4xx, anything else is
// logged and becomes a generic 500. what names the thing ("case", "suspect").
func (h *Handler) storeFailed(w http.ResponseWriter, r *http.Request, err error, what, logMsg string) {
	switch {
	case errors.Is(err, ErrNotFound):
		utils.ErrorResponse(w, http.StatusNotFound, what+" not found", nil)
	case errors.Is(err, ErrInvalid):
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed", nil)
	case errors.Is(err, ErrInUse):
		utils.ErrorResponse(w, http.StatusConflict, "players have played this case; unpublish it instead of deleting it", nil)
	case errors.Is(err, ErrNameTaken):
		utils.ErrorResponse(w, http.StatusConflict, "another suspect in this case has that name", map[string]string{"name": "is already used in this case"})
	case errors.Is(err, ErrOrderTaken):
		utils.ErrorResponse(w, http.StatusConflict, "another scene in this case has that position", map[string]string{"display_order": "is already used in this case"})
	case errors.Is(err, ErrNotReady):
		utils.ErrorResponse(w, http.StatusConflict, "a published case needs at least 2 suspects and exactly one culprit",
			map[string]string{"is_published": "needs at least 2 suspects and exactly one culprit"})
	case errors.Is(err, ErrStaleOrder):
		utils.ErrorResponse(w, http.StatusConflict, "the case list changed since you loaded it; reload and try again", nil)
	case errors.Is(err, ErrNeedsKeywords):
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed",
			map[string]string{"confession_keywords": "add at least one keyword, or the secrets are in the prompt from the first question"})
	default:
		utils.LogErrorCtx(r.Context(), logMsg, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not save the "+what, nil)
	}
}

// ownedImage matches the objects Upload creates. Only those are ever deleted: case
// art uploaded by assetctl (cases/case-01/...) or an external URL is left alone.
var ownedImage = regexp.MustCompile(`^cases/[0-9a-f-]{36}/(thumbnail|scenes|suspects)/[0-9a-f-]{36}\.webp$`)

// dropImage deletes the object behind oldURL from storage when it was replaced
// (newURL differs) and was uploaded through the admin panel. Best effort: a
// failure is logged, never returned, and leaves an orphan object behind.
func (h *Handler) dropImage(ctx context.Context, oldURL, newURL string) {
	if h.images == nil || oldURL == "" || oldURL == newURL {
		return
	}
	prefix := h.images.PublicURL("")
	if !strings.HasPrefix(oldURL, prefix) {
		return
	}
	path := strings.TrimPrefix(oldURL, prefix)
	if !ownedImage.MatchString(path) {
		return
	}
	dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := h.images.Delete(dctx, path); err != nil {
		utils.LogErrorCtx(ctx, "delete replaced image failed", "path", path, "err", err)
	}
}

// checkModel validates the model id's format, then asks OpenRouter that it exists
// and outputs text only. It writes the error response itself.
func (h *Handler) checkModel(w http.ResponseWriter, r *http.Request, model string) bool {
	if !llm.ValidModelID(model) {
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed",
			map[string]string{"llm_model": "must be an OpenRouter model id like google/gemma-4-31b-it (only :free, :nitro or :floor variants)"})
		return false
	}
	if !h.models.Enabled() {
		utils.LogWarnCtx(r.Context(), "OPENROUTER_API_KEY not set; llm_model saved after a format check only", "llm_model", model)
		return true
	}
	err := h.models.CheckTextModel(r.Context(), model)
	switch {
	case err == nil:
		return true
	case errors.Is(err, llm.ErrUnknownModel):
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed", map[string]string{"llm_model": "OpenRouter has no model with this id"})
	case errors.Is(err, llm.ErrNotTextOnly):
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed", map[string]string{"llm_model": "the model must output text only"})
	case errors.Is(err, llm.ErrTooExpensive):
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed", map[string]string{"llm_model": "the model costs too much per token for interrogations"})
	default:
		utils.LogErrorCtx(r.Context(), "model check failed", "llm_model", model, "err", err)
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "could not verify the model with OpenRouter", nil)
	}
	return false
}
