package dashboard

import (
	"context"
	"errors"
	"net/http"

	"backend/internal/user"
	"backend/pkg/utils"
)

// leaderboardSize is how many players the leaderboard shows.
const leaderboardSize = 10

// Balancer reads the live token balance; *wallet.Service implements it.
type Balancer interface {
	Balance(ctx context.Context, userID string) (int64, error)
}

type Handler struct {
	store   Store
	balance Balancer
	board   Board
	userID  func(context.Context) string // auth.UserID in production
}

func NewHandler(store Store, balance Balancer, board Board, userID func(context.Context) string) *Handler {
	return &Handler{store: store, balance: balance, board: board, userID: userID}
}

type statsResp struct {
	Trophies        int64   `json:"trophies"`
	Tokens          int64   `json:"tokens"`
	CasesSolved     int64   `json:"cases_solved"`
	ActiveCaseTitle *string `json:"active_case_title"` // null when no case is in progress
}

type rankedPlayer struct {
	Rank     int    `json:"rank"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Trophies int64  `json:"trophies"`
}

type activeCaseResp struct {
	CaseID     *string `json:"case_id"`     // null when the user is free to pick a story
	StoryTitle *string `json:"story_title"` // title of that story, null when free
}

// Stories handles GET /api/dashboard/stories.
func (h *Handler) Stories(w http.ResponseWriter, r *http.Request) {
	stories, err := h.store.ListStories(r.Context())
	if err != nil {
		utils.LogErrorCtx(r.Context(), "list stories failed", "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not load stories", nil)
		return
	}
	utils.SuccessResponse(w, http.StatusOK, stories)
}

// Stats handles GET /api/dashboard/stats: the caller's trophies, solved-case
// count and in-progress case (Postgres) and live token balance (Redis,
// refilled from Postgres on a miss).
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	ctx, uid := r.Context(), h.userID(r.Context())

	st, err := h.store.Stats(ctx, uid)
	if err != nil {
		h.userError(w, r, "load stats failed", uid, err)
		return
	}
	tokens, err := h.balance.Balance(ctx, uid)
	if err != nil {
		h.userError(w, r, "load token balance failed", uid, err)
		return
	}
	active, err := h.store.ActiveCase(ctx, uid)
	if err != nil {
		h.userError(w, r, "load active case failed", uid, err)
		return
	}
	resp := statsResp{Trophies: st.Trophies, Tokens: tokens, CasesSolved: st.CasesSolved}
	if active != nil {
		resp.ActiveCaseTitle = &active.Title
	}
	utils.SuccessResponse(w, http.StatusOK, resp)
}

// Leaderboard handles GET /api/dashboard/leaderboard: the top players by
// trophies from the Redis sorted set, or from Postgres if Redis has nothing.
func (h *Handler) Leaderboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	entries, err := h.board.Top(ctx, leaderboardSize)
	if err != nil {
		utils.LogErrorCtx(ctx, "leaderboard read failed; using postgres", "err", err)
	}
	if err != nil || len(entries) == 0 {
		players, err := h.store.TopPlayers(ctx, leaderboardSize)
		if err != nil {
			utils.LogErrorCtx(ctx, "leaderboard fallback failed", "err", err)
			utils.ErrorResponse(w, http.StatusInternalServerError, "could not load the leaderboard", nil)
			return
		}
		out := make([]rankedPlayer, 0, len(players))
		for i, p := range players {
			out = append(out, rankedPlayer{Rank: i + 1, UserID: p.ID, Username: p.Username, Trophies: p.Trophies})
		}
		utils.SuccessResponse(w, http.StatusOK, out)
		return
	}

	ids := make([]string, len(entries))
	for i, e := range entries {
		ids[i] = e.UserID
	}
	names, err := h.store.Usernames(ctx, ids)
	if err != nil {
		utils.LogErrorCtx(ctx, "leaderboard usernames failed", "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not load the leaderboard", nil)
		return
	}
	out := make([]rankedPlayer, 0, len(entries))
	for _, e := range entries {
		name, ok := names[e.UserID]
		if !ok {
			continue // deleted account still in the set
		}
		out = append(out, rankedPlayer{Rank: len(out) + 1, UserID: e.UserID, Username: name, Trophies: e.Trophies})
	}
	utils.SuccessResponse(w, http.StatusOK, out)
}

// ActiveCase handles GET /api/dashboard/active-case. case_id (and the story's
// title) identify the case the caller is locked into; both are null when they
// may choose a new story.
func (h *Handler) ActiveCase(w http.ResponseWriter, r *http.Request) {
	uid := h.userID(r.Context())
	active, err := h.store.ActiveCase(r.Context(), uid)
	if err != nil {
		utils.LogErrorCtx(r.Context(), "load active case failed", "user_id", uid, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not check your active case", nil)
		return
	}
	var resp activeCaseResp
	if active != nil {
		resp = activeCaseResp{CaseID: &active.CaseID, StoryTitle: &active.Title}
	}
	utils.SuccessResponse(w, http.StatusOK, resp)
}

// userError answers for a failed per-user lookup: 404 if the account is gone,
// otherwise a logged 500.
func (h *Handler) userError(w http.ResponseWriter, r *http.Request, msg, uid string, err error) {
	if errors.Is(err, user.ErrNotFound) {
		utils.ErrorResponse(w, http.StatusNotFound, "user not found", nil)
		return
	}
	utils.LogErrorCtx(r.Context(), msg, "user_id", uid, "err", err)
	utils.ErrorResponse(w, http.StatusInternalServerError, "could not load your stats", nil)
}
