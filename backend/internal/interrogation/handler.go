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
	svc      *Service
	userID   func(context.Context) string // auth.UserID in production
	clientIP func(*http.Request) string   // utils.ClientIP with the trusted proxies; nil: no per-address limit
}

// HandlerOption customises a Handler.
type HandlerOption func(*Handler)

// WithClientIP sets how the caller's address is read, for the per-address limit.
func WithClientIP(f func(*http.Request) string) HandlerOption {
	return func(h *Handler) { h.clientIP = f }
}

func NewHandler(svc *Service, userID func(context.Context) string, opts ...HandlerOption) *Handler {
	h := &Handler{svc: svc, userID: userID}
	for _, o := range opts {
		o(h)
	}
	return h
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
	uid := h.userID(r.Context())
	if h.clientIP != nil {
		// Before the body is read or anything is looked up.
		if err := h.svc.AdmitIP(r.Context(), h.clientIP(r)); err != nil {
			h.fail(w, r, uid, p.CaseID, err)
			return
		}
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

	res, err := h.svc.GenerateSuspectResponse(r.Context(), Input{UserID: uid, StoryID: p.CaseID, SuspectID: p.SuspectID, Question: q})
	if err != nil {
		h.fail(w, r, uid, p.CaseID, err)
		return
	}
	utils.SuccessResponse(w, http.StatusOK, res)
}

// fail answers a refused or failed question. Nothing here echoes the error text of
// the model or the database.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, uid, caseID string, err error) {
	ctx := r.Context()
	var (
		blocked  *BlockedError
		limit    *LimitError
		refunded *RefundedError
		failed   *FailedError
	)
	switch {
	case errors.As(err, &blocked):
		utils.LogWarnCtx(ctx, "question blocked by moderation", "user_id", uid, "story_id", caseID, "verdict", string(blocked.Verdict))
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, llm.OffTopicReply, map[string]string{"question": "off_topic"})
	case errors.As(err, &limit):
		utils.LogWarnCtx(ctx, "interrogation rate limited", "user_id", uid, "scope", limit.Scope)
		w.Header().Set("Retry-After", secondsString(limit.RetryAfter))
		utils.ErrorResponse(w, http.StatusTooManyRequests, limitMessage(limit.Scope),
			map[string]any{"limit": limit.Scope, "retry_after_seconds": int(limit.RetryAfter.Seconds())})
	case errors.Is(err, ErrBusy):
		w.Header().Set("Retry-After", "5")
		utils.ErrorResponse(w, http.StatusTooManyRequests, "The suspect is still answering your last question.", map[string]string{"limit": "busy"})
	case errors.Is(err, ErrNotInCase):
		utils.ErrorResponse(w, http.StatusForbidden, "Start this case before interrogating its suspects", nil)
	case errors.Is(err, ErrNotFound):
		utils.ErrorResponse(w, http.StatusNotFound, "case not found", nil)
	case errors.Is(err, ErrSuspectNotFound):
		utils.ErrorResponse(w, http.StatusNotFound, "suspect not found", nil)
	case errors.Is(err, ErrNoTokens):
		utils.ErrorResponse(w, http.StatusTooManyRequests, "Out of tokens", map[string]string{"tokens": "out_of_tokens"})
	case errors.Is(err, llm.ErrDisabled):
		utils.LogErrorCtx(ctx, "interrogation unavailable: OPENROUTER_API_KEY not set")
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "Interrogations are unavailable right now", nil)
	case errors.Is(err, ErrUnavailable):
		utils.LogWarnCtx(ctx, "interrogation closed: budget, breaker or concurrency cap", "user_id", uid, "err", err)
		w.Header().Set("Retry-After", "30")
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "The suspects are not taking questions right now. Try again shortly.", nil)
	case errors.As(err, &refunded) && errors.Is(err, llm.ErrTimeout):
		utils.LogWarnCtx(ctx, "suspect model timed out", "user_id", uid, "story_id", caseID, "err", err)
		utils.ErrorResponse(w, http.StatusGatewayTimeout, "The suspect did not answer in time. Your token was refunded.", map[string]string{"token": "refunded"})
	case errors.As(err, &refunded):
		utils.LogErrorCtx(ctx, "suspect model failed", "user_id", uid, "story_id", caseID, "err", err)
		utils.ErrorResponse(w, http.StatusBadGateway, "The suspect could not answer. Your token was refunded.", map[string]string{"token": "refunded"})
	case errors.As(err, &failed):
		utils.LogErrorCtx(ctx, "suspect model failed; token kept", "user_id", uid, "story_id", caseID, "err", err)
		status := http.StatusBadGateway
		if errors.Is(err, llm.ErrTimeout) {
			status = http.StatusGatewayTimeout
		}
		utils.ErrorResponse(w, status, "The suspect could not give a usable answer.", map[string]string{"token": "spent"})
	default:
		utils.LogErrorCtx(ctx, "interrogation failed", "user_id", uid, "story_id", caseID, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not question the suspect", nil)
	}
}

func limitMessage(scope string) string {
	switch scope {
	case "daily":
		return "You have questioned the suspects enough for today. Come back tomorrow."
	case "strikes":
		return "The suspects will not talk to you for a while."
	}
	return "Slow down, Detective. Give the suspect a moment."
}
