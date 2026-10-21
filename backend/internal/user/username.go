package user

import (
	"errors"
	"net/http"

	"backend/pkg/utils"
)

type usernameReq struct {
	Username string `json:"username" validate:"required,username"`
}

// SetUsername handles PUT /api/user/username (protected by auth.Middleware). The unique
// index on lower(username) decides races: a name taken in the meantime is a 409.
func (h *Handler) SetUsername(w http.ResponseWriter, r *http.Request) {
	uid := h.userID(r.Context())
	var req usernameReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	err := h.store.SetUsername(r.Context(), uid, req.Username)
	switch {
	case errors.Is(err, ErrUsernameTaken):
		utils.ErrorResponse(w, http.StatusConflict, "that username is already taken",
			map[string]string{"username": "is already taken"})
		return
	case errors.Is(err, ErrNotFound):
		utils.ErrorResponse(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	case err != nil:
		utils.LogErrorCtx(r.Context(), "save username failed", "user_id", uid, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not save username", nil)
		return
	}
	utils.LogInfoCtx(r.Context(), "username changed", "user_id", uid)
	utils.SuccessResponse(w, http.StatusOK, map[string]string{"username": req.Username})
}
