package interrogation

import (
	"context"
	"errors"
	"net/http"
	"unicode/utf8"

	"backend/internal/llm"
	"backend/pkg/utils"
)

type Handler struct {
	svc    *Service
	userID func(context.Context) string // auth.UserID in production
}

func NewHandler(svc *Service, userID func(context.Context) string) *Handler {
	return &Handler{svc: svc, userID: userID}
}

type pathParams struct {
	CaseID    string `validate:"required,uuid"`
	SuspectID string `validate:"required,uuid"`
}

type askReq struct {
	Question string `json:"question" validate:"required,max=500"` // llm.MaxQuestionLength
}

// Ask handles POST /api/cases/{id}/suspects/{suspect_id}/interrogate.
func (h *Handler) Ask(w http.ResponseWriter, r *http.Request) {
	p := pathParams{CaseID: r.PathValue("id"), SuspectID: r.PathValue("suspect_id")}
	if errs := utils.Validate(p); errs != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid case or suspect id", nil)
		return
	}
	var req askReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	q := llm.CleanQuestion(req.Question)
	if q == "" || utf8.RuneCountInString(q) > llm.MaxQuestionLength {
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed",
			map[string]string{"question": "must be between 1 and 500 characters"})
		return
	}

	uid := h.userID(r.Context())
	res, err := h.svc.GenerateSuspectResponse(r.Context(), Input{UserID: uid, StoryID: p.CaseID, SuspectID: p.SuspectID, Question: q})
	var refunded *RefundedError
	switch {
	case err == nil:
		utils.SuccessResponse(w, http.StatusOK, res)
	case errors.Is(err, ErrBlocked):
		utils.LogInfoCtx(r.Context(), "question blocked by moderation", "user_id", uid, "story_id", p.CaseID)
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, llm.OffTopicReply, map[string]string{"question": "off_topic"})
	case errors.Is(err, ErrNotInCase):
		utils.ErrorResponse(w, http.StatusForbidden, "Start this case before interrogating its suspects", nil)
	case errors.Is(err, ErrNotFound):
		utils.ErrorResponse(w, http.StatusNotFound, "case not found", nil)
	case errors.Is(err, ErrSuspectNotFound):
		utils.ErrorResponse(w, http.StatusNotFound, "suspect not found", nil)
	case errors.Is(err, ErrNoTokens):
		utils.ErrorResponse(w, http.StatusForbidden, "Out of tokens", map[string]string{"tokens": "out_of_tokens"})
	case errors.Is(err, llm.ErrDisabled):
		utils.LogErrorCtx(r.Context(), "interrogation unavailable: OPENROUTER_API_KEY not set")
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "Interrogations are unavailable right now", nil)
	case errors.As(err, &refunded) && errors.Is(err, llm.ErrTimeout):
		utils.LogErrorCtx(r.Context(), "suspect model timed out", "user_id", uid, "story_id", p.CaseID, "err", err)
		utils.ErrorResponse(w, http.StatusGatewayTimeout, "The suspect did not answer in time. Your token was refunded.", nil)
	case errors.As(err, &refunded):
		utils.LogErrorCtx(r.Context(), "suspect model failed", "user_id", uid, "story_id", p.CaseID, "err", err)
		utils.ErrorResponse(w, http.StatusBadGateway, "The suspect could not answer. Your token was refunded.", nil)
	default:
		utils.LogErrorCtx(r.Context(), "interrogation failed", "user_id", uid, "story_id", p.CaseID, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not question the suspect", nil)
	}
}
