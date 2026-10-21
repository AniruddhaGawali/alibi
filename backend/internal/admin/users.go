package admin

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"backend/pkg/utils"
)

// searchLimit is how many players one search returns.
const searchLimit = 20

// SearchUsers handles GET /api/admin/users?q=...: up to 20 players whose username or
// email contains q (newest accounts when q is empty).
func (h *Handler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if utf8.RuneCountInString(q) > 100 {
		utils.ErrorResponse(w, http.StatusBadRequest, "search is too long", map[string]string{"q": "must be at most 100"})
		return
	}
	players, err := h.store.SearchUsers(r.Context(), q, searchLimit)
	if err != nil {
		utils.LogErrorCtx(r.Context(), "search users failed", "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not search players", nil)
		return
	}
	utils.SuccessResponse(w, http.StatusOK, players)
}

type grantReq struct {
	// A player never holds more than the max_tokens setting (at most user.TokenCeiling); the wallet stops there.
	Amount int64 `json:"amount" validate:"required,min=1,max=100"`
}

type grantResp struct {
	UserID       string `json:"user_id"`
	TokenBalance int64  `json:"token_balance"`
	Granted      int64  `json:"granted"` // what was actually added: the balance stops at the cap
	Capped       bool   `json:"capped"`  // true when part of the amount did not fit
}

// GrantTokens handles POST /api/admin/users/{id}/tokens {amount}: adds question
// tokens to one player now, up to the max_tokens a player can hold. The daily reset still
// sets everyone back to the daily allowance, so granted tokens do not carry over.
func (h *Handler) GrantTokens(w http.ResponseWriter, r *http.Request) {
	if h.tokens == nil {
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "granting tokens is not available", nil)
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req grantReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	p, err := h.store.GetUser(r.Context(), id)
	if err != nil {
		h.storeFailed(w, r, err, "player", "load player failed")
		return
	}
	balance, err := h.tokens.Grant(r.Context(), id, req.Amount)
	if err != nil {
		utils.LogErrorCtx(r.Context(), "grant tokens failed", "user_id", id, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not grant the tokens", nil)
		return
	}
	// The read before the grant can be a moment old; never report a negative grant.
	granted := max(balance-p.TokenBalance, 0)
	utils.LogInfoCtx(r.Context(), "admin granted tokens", "admin_id", h.userID(r.Context()), "user_id", id,
		"requested", req.Amount, "granted", granted, "balance", balance)
	utils.SuccessResponse(w, http.StatusOK, grantResp{
		UserID: id, TokenBalance: balance, Granted: granted, Capped: granted < req.Amount && balance >= h.tokens.MaxTokens(r.Context()),
	})
}

type resetLimitsResp struct {
	UserID string `json:"user_id"`
}

// ResetLimits handles POST /api/admin/users/{id}/reset-limits: clears the player's
// daily interrogation allowance, refund count, per-minute window and strike lockout
// so they can question suspects again today. Their token balance is not changed.
func (h *Handler) ResetLimits(w http.ResponseWriter, r *http.Request) {
	if h.limits == nil {
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "resetting limits is not available", nil)
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if _, err := h.store.GetUser(r.Context(), id); err != nil {
		h.storeFailed(w, r, err, "player", "load player failed")
		return
	}
	if err := h.limits.ResetLimits(r.Context(), id); err != nil {
		utils.LogErrorCtx(r.Context(), "reset interrogation limits failed", "user_id", id, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not reset the limits", nil)
		return
	}
	utils.LogInfoCtx(r.Context(), "admin reset interrogation limits", "admin_id", h.userID(r.Context()), "user_id", id)
	utils.SuccessResponse(w, http.StatusOK, resetLimitsResp{UserID: id})
}
