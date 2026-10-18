package cases

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode"

	"backend/pkg/utils"
	"github.com/google/uuid"
)

// LogLookup finds interrogation log entries; *chat.MongoStore implements it.
type LogLookup interface {
	// SuspectOf returns the suspect an entry was with, and whether the entry exists
	// for this user in this case (a case is a story id).
	SuspectOf(ctx context.Context, userID, caseID, messageID string) (suspectID string, found bool, err error)
}

type Handler struct {
	store  Store
	userID func(context.Context) string // auth.UserID in production
	logs   LogLookup                    // nil: notes cannot be tied to log entries
}

func NewHandler(store Store, userID func(context.Context) string) *Handler {
	return &Handler{store: store, userID: userID}
}

// WithLogs lets notes be tied to interrogation log entries, checked against logs.
func (h *Handler) WithLogs(l LogLookup) *Handler {
	h.logs = l
	return h
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
	// Ids are compared as strings, so use the canonical lower-case form.
	parsed, err := uuid.Parse(id)
	if err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid case id", map[string]string{"id": "must be a valid UUID"})
		return "", false
	}
	return parsed.String(), true
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
	b, err := h.store.Briefing(r.Context(), uid, id)
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

// Hints handles POST /api/cases/{id}/hints: reveals the player's next hint. Nothing is
// charged now: hint n takes n trophies off the reward if they solve the case.
func (h *Handler) Hints(w http.ResponseWriter, r *http.Request) {
	id, ok := caseID(w, r)
	if !ok {
		return
	}
	uid := h.userID(r.Context())
	hint, err := h.store.RevealHint(r.Context(), uid, id)
	switch {
	case errors.Is(err, ErrNotLocked):
		utils.ErrorResponse(w, http.StatusForbidden, "You are not investigating this case", nil)
	case errors.Is(err, ErrNoHints):
		utils.ErrorResponse(w, http.StatusBadRequest, "No hints remaining", nil)
	case err != nil:
		utils.LogErrorCtx(r.Context(), "reveal hint failed", "user_id", uid, "case_id", id, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not reveal a hint", nil)
	default:
		utils.LogInfoCtx(r.Context(), "hint revealed", "user_id", uid, "case_id", id, "hint", hint.Number)
		utils.SuccessResponse(w, http.StatusOK, hint)
	}
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
	case errors.Is(err, ErrClosed):
		utils.ErrorResponse(w, http.StatusConflict, "You have already closed this case", nil)
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

// Result handles GET /api/cases/{id}/result: the caller's own settled accusation with
// the truth of the crime. 404 while the case is still open to them, so the solution
// cannot be read early.
func (h *Handler) Result(w http.ResponseWriter, r *http.Request) {
	id, ok := caseID(w, r)
	if !ok {
		return
	}
	uid := h.userID(r.Context())
	res, err := h.store.Result(r.Context(), uid, id)
	if errors.Is(err, ErrNotFound) {
		utils.ErrorResponse(w, http.StatusNotFound, "case not closed", nil)
		return
	}
	if err != nil {
		utils.LogErrorCtx(r.Context(), "load case result failed", "user_id", uid, "case_id", id, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not load the result", nil)
		return
	}
	utils.SuccessResponse(w, http.StatusOK, res)
}

type noteIDParam struct {
	NoteID string `validate:"required,uuid"`
}

type noteReq struct {
	Body      string  `json:"body" validate:"required,max=1000"`
	SuspectID *string `json:"suspect_id" validate:"omitempty,uuid"`
	// ChatMessageID ties the note to one interrogation log entry (its id from the chat history).
	ChatMessageID *string `json:"chat_message_id" validate:"omitempty,min=1,max=64"`
}

// Notes handles GET /api/cases/{id}/notes: the caller's own notes for the case.
func (h *Handler) Notes(w http.ResponseWriter, r *http.Request) {
	id, ok := caseID(w, r)
	if !ok {
		return
	}
	uid := h.userID(r.Context())
	notes, err := h.store.Notes(r.Context(), uid, id)
	if err != nil {
		utils.LogErrorCtx(r.Context(), "load notes failed", "user_id", uid, "case_id", id, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not load your notes", nil)
		return
	}
	utils.SuccessResponse(w, http.StatusOK, notes)
}

// AddNote handles POST /api/cases/{id}/notes {"body": "...", "suspect_id": "<uuid>|null"}.
func (h *Handler) AddNote(w http.ResponseWriter, r *http.Request) {
	id, ok := caseID(w, r)
	if !ok {
		return
	}
	var req noteReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	body := cleanNote(req.Body)
	if body == "" {
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed", map[string]string{"body": "is required"})
		return
	}
	uid := h.userID(r.Context())
	suspectID := req.SuspectID
	if req.ChatMessageID != nil {
		// The entry must be the caller's own, from this case. The note is then about
		// the suspect of that entry; a different suspect in the request is refused.
		if h.logs == nil {
			utils.LogErrorCtx(r.Context(), "note on a log entry refused: chat history is not configured", "user_id", uid, "case_id", id)
			utils.ErrorResponse(w, http.StatusServiceUnavailable, "Notes on log entries are unavailable right now", nil)
			return
		}
		entrySuspect, found, err := h.logs.SuspectOf(r.Context(), uid, id, *req.ChatMessageID)
		if err != nil {
			utils.LogErrorCtx(r.Context(), "look up log entry failed", "user_id", uid, "case_id", id, "err", err)
			utils.ErrorResponse(w, http.StatusInternalServerError, "could not save your note", nil)
			return
		}
		if !found {
			utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed", map[string]string{"chat_message_id": "is not a log entry of this case"})
			return
		}
		if suspectID != nil && *suspectID != entrySuspect {
			utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed", map[string]string{"suspect_id": "does not match the log entry"})
			return
		}
		suspectID = &entrySuspect
	}
	note, err := h.store.AddNote(r.Context(), uid, id, suspectID, req.ChatMessageID, body)
	switch {
	case errors.Is(err, ErrNotFound):
		utils.ErrorResponse(w, http.StatusNotFound, "case not found", nil)
	case errors.Is(err, ErrBadSuspect):
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed", map[string]string{"suspect_id": "is not a suspect of this case"})
	case errors.Is(err, ErrNotesFull):
		utils.ErrorResponse(w, http.StatusConflict, "You have reached the limit of notes for this case. Delete some to add more.", nil)
	case errors.Is(err, ErrUserGone):
		utils.ErrorResponse(w, http.StatusUnauthorized, "unauthorized", nil)
	case err != nil:
		utils.LogErrorCtx(r.Context(), "add note failed", "user_id", uid, "case_id", id, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not save your note", nil)
	default:
		utils.SuccessResponse(w, http.StatusCreated, note)
	}
}

// DeleteNote handles DELETE /api/cases/{id}/notes/{note_id}. Only the author can delete a note.
func (h *Handler) DeleteNote(w http.ResponseWriter, r *http.Request) {
	noteID := r.PathValue("note_id")
	if errs := utils.Validate(noteIDParam{NoteID: noteID}); errs != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid note id", map[string]string{"note_id": "must be a valid UUID"})
		return
	}
	uid := h.userID(r.Context())
	err := h.store.DeleteNote(r.Context(), uid, noteID)
	if errors.Is(err, ErrNotFound) {
		utils.ErrorResponse(w, http.StatusNotFound, "note not found", nil)
		return
	}
	if err != nil {
		utils.LogErrorCtx(r.Context(), "delete note failed", "user_id", uid, "note_id", noteID, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not delete the note", nil)
		return
	}
	utils.SuccessResponse(w, http.StatusOK, map[string]string{"message": "note deleted"})
}

// cleanNote trims a note and drops control characters (NUL included, which
// Postgres refuses in text) other than line breaks and tabs.
func cleanNote(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}
