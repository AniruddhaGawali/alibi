package chat

import (
	"context"
	"net/http"

	"backend/pkg/utils"
)

// Reader reads stored exchanges; *MongoStore implements it.
type Reader interface {
	History(ctx context.Context, userID, caseID, suspectID string, limit int) ([]Message, error)
}

type Handler struct {
	store  Reader
	userID func(context.Context) string // auth.UserID in production
}

func NewHandler(store Reader, userID func(context.Context) string) *Handler {
	return &Handler{store: store, userID: userID}
}

type historyParams struct {
	CaseID    string `validate:"required,uuid"`
	SuspectID string `validate:"required,uuid"`
}

// History handles GET /api/chat/{case_id}/{suspect_id}: the caller's own
// exchanges with that suspect, oldest first (the latest MaxHistory of them).
// The user id always comes from the JWT, so nobody can read another player's chat.
func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	p := historyParams{CaseID: r.PathValue("case_id"), SuspectID: r.PathValue("suspect_id")}
	if errs := utils.Validate(p); errs != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid case or suspect id", nil)
		return
	}
	uid := h.userID(r.Context())
	msgs, err := h.store.History(r.Context(), uid, p.CaseID, p.SuspectID, MaxHistory)
	if err != nil {
		utils.LogErrorCtx(r.Context(), "load chat history failed", "user_id", uid, "case_id", p.CaseID, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not load the chat history", nil)
		return
	}
	utils.SuccessResponse(w, http.StatusOK, msgs)
}
