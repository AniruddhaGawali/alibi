package settings

import (
	"net/http"

	"backend/pkg/utils"
)

// Rules is the public part of the settings: what the landing and rules pages tell
// visitors about the token economy.
type Rules struct {
	DailyTokens int    `json:"daily_tokens"`
	MaxTokens   int    `json:"max_tokens"`
	ResetTime   string `json:"token_reset_time"`
	Timezone    string `json:"token_reset_timezone"`
}

// PublicRules handles GET /api/game/rules (public, no login): the daily allowance, the
// most a player can hold and the reset time. Served from the in-memory copy, so it
// costs no database work.
func (s *Service) PublicRules(w http.ResponseWriter, r *http.Request) {
	c := s.Current(r.Context())
	utils.SuccessResponse(w, http.StatusOK, Rules{DailyTokens: c.DailyTokens, MaxTokens: c.MaxTokens, ResetTime: c.ResetTime, Timezone: c.Timezone})
}
