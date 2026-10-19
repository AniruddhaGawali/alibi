package admin

import (
	"errors"
	"net/http"

	"backend/internal/settings"
	"backend/pkg/utils"
)

type configReq struct {
	DailyTokens int    `json:"daily_tokens" validate:"required,min=1,max=10"` // settings.MaxDailyTokens
	ResetTime   string `json:"token_reset_time" validate:"required,clock"`
	Timezone    string `json:"token_reset_timezone" validate:"required,timezone"`
}

// GetConfig handles GET /api/admin/config.
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	if h.settings == nil {
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "settings are not available", nil)
		return
	}
	utils.SuccessResponse(w, http.StatusOK, h.settings.Current(r.Context()))
}

// UpdateConfig handles PUT /api/admin/config. The new daily allowance applies from
// the next reset (balances are not touched now); the new time applies from the
// next check of the reset job, within a minute. Moving the time earlier than now
// on a day that has not been reset yet resets at once. Other replicas pick the
// change up within settings' cache time.
func (h *Handler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	if h.settings == nil {
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "settings are not available", nil)
		return
	}
	var req configReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	uid := h.userID(r.Context())
	s, err := h.settings.Update(r.Context(), settings.Settings{DailyTokens: req.DailyTokens, ResetTime: req.ResetTime, Timezone: req.Timezone}, uid)
	if errors.Is(err, settings.ErrNotFound) {
		utils.LogErrorCtx(r.Context(), "settings row missing", "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not save the settings", nil)
		return
	}
	if err != nil {
		utils.LogErrorCtx(r.Context(), "update settings failed", "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not save the settings", nil)
		return
	}
	utils.LogInfoCtx(r.Context(), "settings updated", "admin_id", uid, "daily_tokens", s.DailyTokens,
		"token_reset_time", s.ResetTime, "token_reset_timezone", s.Timezone)
	utils.SuccessResponse(w, http.StatusOK, s)
}
