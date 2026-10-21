package admin

import (
	"net/http"

	"backend/pkg/utils"
)

// The persona fields are what llm.BuildSystemPrompt assembles into the suspect's
// system prompt; the hidden truth and confession triggers only enter it once a
// question matches one of the confession keywords (llm.Unlocked).
type createSuspectReq struct {
	Name                 string   `json:"name" validate:"required,max=100"`
	Role                 string   `json:"role" validate:"required,max=200"`
	AvatarURL            string   `json:"avatar_url" validate:"omitempty,https_url,max=2048"`
	InitialTestimony     string   `json:"initial_testimony" validate:"required,max=4000"`
	PublicKnowledge      string   `json:"public_knowledge" validate:"max=4000"` // shown to investigators only, never a secret
	IsCulprit            bool     `json:"is_culprit"`                           // marking one moves the flag off any other suspect
	DisplayOrder         int32    `json:"display_order" validate:"min=0,max=1000"`
	RelationshipToVictim string   `json:"relationship_to_victim" validate:"max=500"`
	Persona              string   `json:"persona" validate:"max=4000"`
	PublicAlibi          string   `json:"public_alibi" validate:"max=4000"`
	HiddenTruth          string   `json:"hidden_truth" validate:"max=6000"`
	ConfessionTriggers   string   `json:"confession_triggers" validate:"max=4000"`
	ConfessionKeywords   []string `json:"confession_keywords" validate:"max=30,dive,keyword"`
}

type updateSuspectReq struct {
	Name                 *string   `json:"name" validate:"omitempty,min=1,max=100"`
	Role                 *string   `json:"role" validate:"omitempty,min=1,max=200"`
	AvatarURL            *string   `json:"avatar_url" validate:"omitempty,https_url,max=2048"`
	InitialTestimony     *string   `json:"initial_testimony" validate:"omitempty,min=1,max=4000"`
	PublicKnowledge      *string   `json:"public_knowledge" validate:"omitempty,max=4000"` // "" clears it
	IsCulprit            *bool     `json:"is_culprit"`
	DisplayOrder         *int32    `json:"display_order" validate:"omitempty,min=0,max=1000"`
	RelationshipToVictim *string   `json:"relationship_to_victim" validate:"omitempty,max=500"`
	Persona              *string   `json:"persona" validate:"omitempty,max=4000"`
	PublicAlibi          *string   `json:"public_alibi" validate:"omitempty,max=4000"`
	HiddenTruth          *string   `json:"hidden_truth" validate:"omitempty,max=6000"`
	ConfessionTriggers   *string   `json:"confession_triggers" validate:"omitempty,max=4000"`
	ConfessionKeywords   *[]string `json:"confession_keywords" validate:"omitempty,max=30,dive,keyword"` // [] clears them
}

// ListSuspects handles GET /api/admin/cases/{id}/suspects, secrets included.
func (h *Handler) ListSuspects(w http.ResponseWriter, r *http.Request) {
	caseID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	c, err := h.store.GetCase(r.Context(), caseID)
	if err != nil {
		h.storeFailed(w, r, err, "case", "list suspects failed")
		return
	}
	utils.SuccessResponse(w, http.StatusOK, c.Suspects)
}

// CreateSuspect handles POST /api/admin/cases/{id}/suspects.
func (h *Handler) CreateSuspect(w http.ResponseWriter, r *http.Request) {
	caseID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req createSuspectReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	s, err := h.store.CreateSuspect(r.Context(), caseID, SuspectInput{
		Name: req.Name, Role: req.Role, AvatarURL: req.AvatarURL, InitialTestimony: req.InitialTestimony,
		PublicKnowledge: req.PublicKnowledge, IsCulprit: req.IsCulprit, DisplayOrder: req.DisplayOrder,
		RelationshipToVictim: req.RelationshipToVictim, Persona: req.Persona, PublicAlibi: req.PublicAlibi,
		HiddenTruth: req.HiddenTruth, ConfessionTriggers: req.ConfessionTriggers, ConfessionKeywords: req.ConfessionKeywords,
	})
	if err != nil {
		h.storeFailed(w, r, err, "suspect", "create suspect failed")
		return
	}
	utils.LogInfoCtx(r.Context(), "suspect created", "admin_id", h.userID(r.Context()), "case_id", caseID, "suspect_id", s.ID)
	utils.SuccessResponse(w, http.StatusCreated, s)
}

// UpdateSuspect handles PATCH /api/admin/cases/{id}/suspects/{suspect_id}. Only the
// fields sent change. A player mid-interrogation sees the change within a minute
// (the prepared prompt is cached that long).
func (h *Handler) UpdateSuspect(w http.ResponseWriter, r *http.Request) {
	caseID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	id, ok := pathID(w, r, "suspect_id")
	if !ok {
		return
	}
	var req updateSuspectReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	if req == (updateSuspectReq{}) {
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed", map[string]string{"body": "send at least one field to change"})
		return
	}
	var keywords []string
	if req.ConfessionKeywords != nil {
		keywords = append([]string{}, *req.ConfessionKeywords...)
	}
	s, old, err := h.store.UpdateSuspect(r.Context(), caseID, id, SuspectPatch{
		Name: req.Name, Role: req.Role, AvatarURL: req.AvatarURL, InitialTestimony: req.InitialTestimony,
		PublicKnowledge: req.PublicKnowledge, IsCulprit: req.IsCulprit, DisplayOrder: req.DisplayOrder,
		RelationshipToVictim: req.RelationshipToVictim, Persona: req.Persona, PublicAlibi: req.PublicAlibi,
		HiddenTruth: req.HiddenTruth, ConfessionTriggers: req.ConfessionTriggers, ConfessionKeywords: keywords,
	})
	if err != nil {
		h.storeFailed(w, r, err, "suspect", "update suspect failed")
		return
	}
	h.dropImage(r.Context(), old, s.AvatarURL)
	utils.LogInfoCtx(r.Context(), "suspect updated", "admin_id", h.userID(r.Context()), "case_id", caseID, "suspect_id", id)
	utils.SuccessResponse(w, http.StatusOK, s)
}

// DeleteSuspect handles DELETE /api/admin/cases/{id}/suspects/{suspect_id}. Past
// accusations of them keep their result with no suspect attached.
func (h *Handler) DeleteSuspect(w http.ResponseWriter, r *http.Request) {
	caseID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	id, ok := pathID(w, r, "suspect_id")
	if !ok {
		return
	}
	avatar, err := h.store.DeleteSuspect(r.Context(), caseID, id)
	if err != nil {
		h.storeFailed(w, r, err, "suspect", "delete suspect failed")
		return
	}
	h.dropImage(r.Context(), avatar, "")
	utils.LogInfoCtx(r.Context(), "suspect deleted", "admin_id", h.userID(r.Context()), "case_id", caseID, "suspect_id", id)
	utils.SuccessResponse(w, http.StatusOK, map[string]string{"id": id})
}

type createSceneReq struct {
	SceneName    string `json:"scene_name" validate:"required,max=200"`
	ImageURL     string `json:"image_url" validate:"required,https_url,max=2048"`
	DisplayOrder int32  `json:"display_order" validate:"min=0,max=1000"`
}

type updateSceneReq struct {
	SceneName    *string `json:"scene_name" validate:"omitempty,min=1,max=200"`
	ImageURL     *string `json:"image_url" validate:"omitempty,https_url,max=2048"`
	DisplayOrder *int32  `json:"display_order" validate:"omitempty,min=0,max=1000"`
}

// CreateScene handles POST /api/admin/cases/{id}/scenes.
func (h *Handler) CreateScene(w http.ResponseWriter, r *http.Request) {
	caseID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req createSceneReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	s, err := h.store.CreateScene(r.Context(), caseID, req.SceneName, req.ImageURL, req.DisplayOrder)
	if err != nil {
		h.storeFailed(w, r, err, "scene", "create scene failed")
		return
	}
	utils.LogInfoCtx(r.Context(), "scene created", "admin_id", h.userID(r.Context()), "case_id", caseID, "scene_id", s.ID)
	utils.SuccessResponse(w, http.StatusCreated, s)
}

// UpdateScene handles PATCH /api/admin/cases/{id}/scenes/{scene_id}.
func (h *Handler) UpdateScene(w http.ResponseWriter, r *http.Request) {
	caseID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	id, ok := pathID(w, r, "scene_id")
	if !ok {
		return
	}
	var req updateSceneReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	if req == (updateSceneReq{}) {
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed", map[string]string{"body": "send at least one field to change"})
		return
	}
	s, old, err := h.store.UpdateScene(r.Context(), caseID, id, ScenePatch{SceneName: req.SceneName, ImageURL: req.ImageURL, DisplayOrder: req.DisplayOrder})
	if err != nil {
		h.storeFailed(w, r, err, "scene", "update scene failed")
		return
	}
	h.dropImage(r.Context(), old, s.ImageURL)
	utils.LogInfoCtx(r.Context(), "scene updated", "admin_id", h.userID(r.Context()), "case_id", caseID, "scene_id", id)
	utils.SuccessResponse(w, http.StatusOK, s)
}

// DeleteScene handles DELETE /api/admin/cases/{id}/scenes/{scene_id}.
func (h *Handler) DeleteScene(w http.ResponseWriter, r *http.Request) {
	caseID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	id, ok := pathID(w, r, "scene_id")
	if !ok {
		return
	}
	img, err := h.store.DeleteScene(r.Context(), caseID, id)
	if err != nil {
		h.storeFailed(w, r, err, "scene", "delete scene failed")
		return
	}
	h.dropImage(r.Context(), img, "")
	utils.LogInfoCtx(r.Context(), "scene deleted", "admin_id", h.userID(r.Context()), "case_id", caseID, "scene_id", id)
	utils.SuccessResponse(w, http.StatusOK, map[string]string{"id": id})
}
