package cases

import (
	"context"
	"errors"
	"net/http"

	"backend/pkg/utils"
)

type Handler struct {
	store  Store
	userID func(context.Context) string // auth.UserID in production
}

func NewHandler(store Store, userID func(context.Context) string) *Handler {
	return &Handler{store: store, userID: userID}
}

type caseIDParam struct {
	ID string `validate:"required,uuid"`
}

// caseID reads and validates the {id} path value; it writes the 400 itself.
func caseID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if errs := utils.Validate(caseIDParam{ID: id}); errs != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid case id", map[string]string{"id": "must be a valid UUID"})
		return "", false
	}
	return id, true
}

// allowed enforces the Active Case Lock: a player locked into one case may not
// read another case's dossier. Players with no lock may read any published case.
// It writes the error response itself.
func (h *Handler) allowed(w http.ResponseWriter, r *http.Request, uid, id string) bool {
	lock, err := h.store.Lock(r.Context(), uid)
	if err != nil {
		utils.LogErrorCtx(r.Context(), "load case lock failed", "user_id", uid, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not load the case", nil)
		return false
	}
	if lock != nil && lock.StoryID != id {
		utils.ErrorResponse(w, http.StatusForbidden, "Finish your active case first", nil)
		return false
	}
	return true
}

// Briefing handles GET /api/cases/{id}/briefing.
func (h *Handler) Briefing(w http.ResponseWriter, r *http.Request) {
	id, ok := caseID(w, r)
	if !ok {
		return
	}
	uid := h.userID(r.Context())
	if !h.allowed(w, r, uid, id) {
		return
	}
	b, err := h.store.Briefing(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		utils.ErrorResponse(w, http.StatusNotFound, "case not found", nil)
		return
	}
	if err != nil {
		utils.LogErrorCtx(r.Context(), "load briefing failed", "case_id", id, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not load the case", nil)
		return
	}
	utils.SuccessResponse(w, http.StatusOK, b)
}

// Suspects handles GET /api/cases/{id}/suspects.
func (h *Handler) Suspects(w http.ResponseWriter, r *http.Request) {
	id, ok := caseID(w, r)
	if !ok {
		return
	}
	uid := h.userID(r.Context())
	if !h.allowed(w, r, uid, id) {
		return
	}
	suspects, err := h.store.Suspects(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		utils.ErrorResponse(w, http.StatusNotFound, "case not found", nil)
		return
	}
	if err != nil {
		utils.LogErrorCtx(r.Context(), "load suspects failed", "case_id", id, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not load the suspects", nil)
		return
	}
	utils.SuccessResponse(w, http.StatusOK, suspects)
}

// Start handles POST /api/cases/{id}/start: it locks the player into the case.
// Starting the case they are already in succeeds again (idempotent); starting a
// different one while locked is a 400.
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	id, ok := caseID(w, r)
	if !ok {
		return
	}
	uid := h.userID(r.Context())
	lock, err := h.store.Start(r.Context(), uid, id)
	switch {
	case errors.Is(err, ErrNotFound):
		utils.ErrorResponse(w, http.StatusNotFound, "case not found", nil)
	case errors.Is(err, ErrLockedElsewhere):
		utils.LogInfoCtx(r.Context(), "case start refused: another case is active", "user_id", uid, "story_id", id)
		utils.ErrorResponse(w, http.StatusBadRequest, "Finish your active case first", nil)
	case errors.Is(err, ErrUserGone):
		utils.ErrorResponse(w, http.StatusUnauthorized, "unauthorized", nil)
	case err != nil:
		utils.LogErrorCtx(r.Context(), "start case failed", "user_id", uid, "story_id", id, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not start the case", nil)
	default:
		utils.LogInfoCtx(r.Context(), "case started", "user_id", uid, "story_id", id, "case_id", lock.CaseID)
		utils.SuccessResponse(w, http.StatusOK, lock)
	}
}
