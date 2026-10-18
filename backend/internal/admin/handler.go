package admin

import (
	"context"
	"errors"
	"net/http"

	"backend/internal/llm"
	"backend/pkg/utils"
)

// ModelChecker confirms a model exists and outputs text only; *llm.Clients
// implements it.
type ModelChecker interface {
	Enabled() bool
	CheckTextModel(ctx context.Context, model string) error
}

type Handler struct {
	store  Store
	models ModelChecker
	userID func(context.Context) string // auth.UserID in production
}

func NewHandler(store Store, models ModelChecker, userID func(context.Context) string) *Handler {
	return &Handler{store: store, models: models, userID: userID}
}

// RequireAdmin lets the request through only for users with is_admin. The flag is
// read from the database on every request, so revoking it takes effect at once.
// Everyone else gets 403.
func (h *Handler) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := h.userID(r.Context())
		ok, err := h.store.IsAdmin(r.Context(), uid)
		if err != nil {
			utils.LogErrorCtx(r.Context(), "admin check failed", "user_id", uid, "err", err)
			utils.ErrorResponse(w, http.StatusInternalServerError, "could not check permissions", nil)
			return
		}
		if !ok {
			utils.LogWarnCtx(r.Context(), "admin route denied", "user_id", uid, "path", r.URL.Path)
			utils.ErrorResponse(w, http.StatusForbidden, "admin access required", nil)
			return
		}
		next(w, r)
	}
}

type createCaseReq struct {
	Title           string `json:"title" validate:"required,max=200"`
	Synopsis        string `json:"synopsis" validate:"required,max=2000"`
	MainPlot        string `json:"main_plot" validate:"max=10000"`
	ThumbnailURL    string `json:"thumbnail_url" validate:"omitempty,https_url,max=2048"`
	DifficultyLevel string `json:"difficulty_level" validate:"required,oneof=easy medium hard"`
	RewardTrophies  int32  `json:"reward_trophies" validate:"min=0,max=1000"`
	IsPublished     bool   `json:"is_published"`                           // false (a draft) when left out
	LLMModel        string `json:"llm_model" validate:"omitempty,max=100"` // llm.DefaultModel when left out
}

type updateCaseReq struct {
	Title           *string `json:"title" validate:"omitempty,min=1,max=200"`
	Synopsis        *string `json:"synopsis" validate:"omitempty,min=1,max=2000"`
	MainPlot        *string `json:"main_plot" validate:"omitempty,max=10000"`
	ThumbnailURL    *string `json:"thumbnail_url" validate:"omitempty,https_url,max=2048"`
	DifficultyLevel *string `json:"difficulty_level" validate:"omitempty,oneof=easy medium hard"`
	RewardTrophies  *int32  `json:"reward_trophies" validate:"omitempty,min=0,max=1000"`
	IsPublished     *bool   `json:"is_published"`
	LLMModel        *string `json:"llm_model" validate:"omitempty,max=100"`
}

// CreateCase handles POST /api/admin/cases.
func (h *Handler) CreateCase(w http.ResponseWriter, r *http.Request) {
	var req createCaseReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	if req.LLMModel == "" {
		req.LLMModel = llm.DefaultModel
	}
	if !h.checkModel(w, r, req.LLMModel) {
		return
	}
	c, err := h.store.CreateCase(r.Context(), CaseInput{
		Title: req.Title, Synopsis: req.Synopsis, MainPlot: req.MainPlot, ThumbnailURL: req.ThumbnailURL,
		DifficultyLevel: req.DifficultyLevel, RewardTrophies: req.RewardTrophies,
		IsPublished: req.IsPublished, LLMModel: req.LLMModel,
	})
	if errors.Is(err, ErrInvalid) {
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed", nil)
		return
	}
	if err != nil {
		utils.LogErrorCtx(r.Context(), "create case failed", "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not create the case", nil)
		return
	}
	utils.LogInfoCtx(r.Context(), "case created", "admin_id", h.userID(r.Context()), "case_id", c.ID, "llm_model", c.LLMModel)
	utils.SuccessResponse(w, http.StatusCreated, c)
}

// UpdateCase handles PATCH /api/admin/cases/{id}. Only the fields sent change.
func (h *Handler) UpdateCase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if errs := utils.Validate(struct {
		ID string `validate:"required,uuid"`
	}{id}); errs != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid case id", map[string]string{"id": "must be a valid UUID"})
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
	if req.LLMModel != nil && !h.checkModel(w, r, *req.LLMModel) {
		return
	}
	c, err := h.store.UpdateCase(r.Context(), id, CasePatch{
		Title: req.Title, Synopsis: req.Synopsis, MainPlot: req.MainPlot, ThumbnailURL: req.ThumbnailURL,
		DifficultyLevel: req.DifficultyLevel, RewardTrophies: req.RewardTrophies,
		IsPublished: req.IsPublished, LLMModel: req.LLMModel,
	})
	switch {
	case errors.Is(err, ErrNotFound):
		utils.ErrorResponse(w, http.StatusNotFound, "case not found", nil)
	case errors.Is(err, ErrInvalid):
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed", nil)
	case err != nil:
		utils.LogErrorCtx(r.Context(), "update case failed", "case_id", id, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not update the case", nil)
	default:
		utils.LogInfoCtx(r.Context(), "case updated", "admin_id", h.userID(r.Context()), "case_id", c.ID, "llm_model", c.LLMModel)
		utils.SuccessResponse(w, http.StatusOK, c)
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
