package accusation

import (
	"context"
	"errors"
	"net/http"

	"backend/internal/dashboard"
	"backend/pkg/utils"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// LeaderboardKey is the Redis sorted set of player scores: member = user id,
// score = trophies. The dashboard reads and rebuilds the same set.
const LeaderboardKey = dashboard.LeaderboardKey

// Leaderboard moves a player's score; *RedisLeaderboard implements it.
type Leaderboard interface {
	// Incr adds delta (negative to subtract) to the player's score.
	Incr(ctx context.Context, userID string, delta int64) error
}

// RedisLeaderboard keeps the Upstash/Redis sorted set in step with trophies.
type RedisLeaderboard struct{ rdb *redis.Client }

func NewRedisLeaderboard(rdb *redis.Client) *RedisLeaderboard { return &RedisLeaderboard{rdb: rdb} }

func (l *RedisLeaderboard) Incr(ctx context.Context, userID string, delta int64) error {
	return l.rdb.ZIncrBy(ctx, LeaderboardKey, float64(delta), userID).Err()
}

type Handler struct {
	store  Store
	board  Leaderboard
	userID func(context.Context) string // auth.UserID in production
}

func NewHandler(store Store, board Leaderboard, userID func(context.Context) string) *Handler {
	return &Handler{store: store, board: board, userID: userID}
}

type accuseReq struct {
	SuspectID string `json:"suspect_id" validate:"required,uuid"`
}

// Accuse handles POST /api/cases/{id}/accuse {"suspect_id": "<uuid>"}. {id} is the
// case (story) the player is locked into. The JWT and its blacklist check are done
// by auth.Middleware before this runs.
func (h *Handler) Accuse(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("id")
	parsed, err := uuid.Parse(caseID)
	if err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid case id", map[string]string{"id": "must be a valid UUID"})
		return
	}
	var req accuseReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	suspect, _ := uuid.Parse(req.SuspectID) // validated above
	uid := h.userID(r.Context())

	out, err := h.store.Accuse(r.Context(), uid, parsed.String(), suspect.String())
	switch {
	case errors.Is(err, ErrNotLocked):
		utils.LogWarnCtx(r.Context(), "accusation refused: no active lock", "user_id", uid, "case_id", parsed.String())
		utils.ErrorResponse(w, http.StatusForbidden, "You are not investigating this case", nil)
		return
	case errors.Is(err, ErrBadSuspect):
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed", map[string]string{"suspect_id": "is not a suspect of this case"})
		return
	case errors.Is(err, ErrNotFound):
		utils.ErrorResponse(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	case err != nil:
		utils.LogErrorCtx(r.Context(), "accusation failed", "user_id", uid, "case_id", parsed.String(), "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not settle your accusation", nil)
		return
	}
	utils.LogInfoCtx(r.Context(), "case closed", "user_id", uid, "case_id", parsed.String(), "correct", out.Correct, "trophies_change", out.Change)

	// Postgres is already committed and is the source of truth, so a Redis failure
	// must not undo or fail the accusation. It is logged, and the leaderboard
	// self-heals from Postgres. WithoutCancel: a dropped client must not skip it.
	if out.Change != 0 {
		if err := h.board.Incr(context.WithoutCancel(r.Context()), uid, out.Change); err != nil {
			utils.LogErrorCtx(r.Context(), "leaderboard update failed", "user_id", uid, "delta", out.Change, "err", err)
		}
	}
	utils.SuccessResponse(w, http.StatusOK, out)
}
