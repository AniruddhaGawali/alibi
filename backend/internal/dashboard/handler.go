package dashboard

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"backend/internal/user"
	"backend/pkg/utils"
)

// leaderboardSize is how many players the leaderboard shows.
const leaderboardSize = 10

// WithBoard makes the leaderboard come from Redis, rebuilt from Postgres when stale.
func (h *Handler) WithBoard(b Board) *Handler {
	h.board = b
	return h
}

// WithMemoryTTL keeps the top players in this process's memory for ttl, so most
// leaderboard views send nothing to Redis or Postgres (Redis is billed per command).
// A score change in this process drops it at once (Forget); one made on another
// replica shows up within ttl. 0 turns it off.
func (h *Handler) WithMemoryTTL(ttl time.Duration) *Handler {
	h.topTTL = ttl
	return h
}

// Forget drops the in-memory top players, so the next view reads the new scores.
func (h *Handler) Forget() {
	h.topMu.Lock()
	h.top = nil
	h.topMu.Unlock()
}

// Balancer reads the live token balance; *wallet.Service implements it.
type Balancer interface {
	Balance(ctx context.Context, userID string) (int64, error)
}

type Handler struct {
	store   Store
	balance Balancer
	userID  func(context.Context) string // auth.UserID in production
	board   Board                        // Redis leaderboard; nil reads Postgres only

	// In-memory top players (WithMemoryTTL). topMu is held while they are refreshed,
	// so an expiry makes one read, not one per waiting request.
	topMu  sync.Mutex
	top    []Player // nil: not loaded or forgotten
	topAt  time.Time
	topTTL time.Duration
	now    func() time.Time
}

func NewHandler(store Store, balance Balancer, userID func(context.Context) string) *Handler {
	return &Handler{store: store, balance: balance, userID: userID, now: time.Now}
}

type statsResp struct {
	Trophies        int64   `json:"trophies"`
	Tokens          int64   `json:"tokens"`
	CasesSolved     int64   `json:"cases_solved"`
	ActiveCaseTitle *string `json:"active_case_title"` // null when no case is in progress
	ActiveStoryID   *string `json:"active_story_id"`   // the story being played, null when none
	// ActiveMusic is the music track of the case in progress ("" = main theme), null
	// when none: the game plays it on every page while the player is in the case.
	ActiveMusic *string `json:"active_music_track"`
	// CaseOutcomes is how each of the caller's closed cases ended ("solved" or "failed"),
	// by story id. It lives here, not in the story list, so that list is the same for
	// everyone and can be cached.
	CaseOutcomes map[string]string `json:"case_outcomes"`
}

type rankedPlayer struct {
	Rank     int    `json:"rank"`
	Username string `json:"username"`
	Trophies int64  `json:"trophies"`
	IsMe     bool   `json:"is_me"` // the caller's own row; internal user ids are never exposed
}

type activeCaseResp struct {
	CaseID     *string `json:"case_id"`     // the play-through id; null when the user is free to pick a story
	StoryID    *string `json:"story_id"`    // the story being played, null when free
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
	outcomes, err := h.store.Outcomes(ctx, uid)
	if err != nil {
		h.userError(w, r, "load case outcomes failed", uid, err)
		return
	}
	resp := statsResp{Trophies: st.Trophies, Tokens: tokens, CasesSolved: st.CasesSolved, CaseOutcomes: outcomes}
	if active != nil {
		resp.ActiveCaseTitle = &active.Title
		resp.ActiveStoryID = &active.StoryID
		resp.ActiveMusic = &active.Music
	}
	utils.SuccessResponse(w, http.StatusOK, resp)
}

// Leaderboard handles GET /api/dashboard/leaderboard: the top players by trophies.
// They come from the Redis sorted set while it is in sync with Postgres. When it
// is not (first read, expired, Redis down) they come from Postgres
// (users_trophies_rank_idx makes that an index scan) and the same read rebuilds
// the set, so Redis is refreshed from the database as the dashboard is used.
func (h *Handler) Leaderboard(w http.ResponseWriter, r *http.Request) {
	ctx, uid := r.Context(), h.userID(r.Context())
	players, err := h.topPlayers(ctx)
	if err != nil {
		utils.LogErrorCtx(ctx, "leaderboard read failed", "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not load the leaderboard", nil)
		return
	}
	out := make([]rankedPlayer, 0, len(players))
	for i, p := range players {
		out = append(out, rankedPlayer{Rank: i + 1, Username: p.Username, Trophies: p.Trophies, IsMe: p.ID == uid})
	}
	utils.SuccessResponse(w, http.StatusOK, out)
}

func (h *Handler) topPlayers(ctx context.Context) ([]Player, error) {
	if h.topTTL <= 0 {
		return h.loadTop(ctx)
	}
	h.topMu.Lock()
	defer h.topMu.Unlock()
	if h.top != nil && h.now().Sub(h.topAt) < h.topTTL {
		return h.top, nil // shared read-only: callers copy it into their response
	}
	players, err := h.loadTop(ctx)
	if err != nil {
		return nil, err
	}
	h.top, h.topAt = players, h.now()
	return players, nil
}

func (h *Handler) loadTop(ctx context.Context) ([]Player, error) {
	if h.board != nil {
		entries, synced, err := h.board.Top(ctx, leaderboardSize)
		if err != nil {
			utils.LogWarnCtx(ctx, "redis leaderboard read failed, using postgres", "err", err)
		} else if synced {
			if players, err := h.named(ctx, entries); err == nil {
				return players, nil
			} else {
				utils.LogWarnCtx(ctx, "leaderboard names failed, using postgres", "err", err)
			}
		}
	}
	if h.board == nil {
		return h.store.TopPlayers(ctx, leaderboardSize)
	}
	all, err := h.store.TopPlayers(ctx, RebuildSize)
	if err != nil {
		return nil, err
	}
	if err := h.board.Rebuild(ctx, all); err != nil {
		utils.LogWarnCtx(ctx, "leaderboard rebuild failed", "err", err)
	}
	return all[:min(len(all), leaderboardSize)], nil
}

// named adds usernames to Redis entries, dropping players that no longer exist.
func (h *Handler) named(ctx context.Context, entries []Entry) ([]Player, error) {
	if len(entries) == 0 {
		return []Player{}, nil
	}
	ids := make([]string, len(entries))
	for i, e := range entries {
		ids[i] = e.ID
	}
	names, err := h.store.Usernames(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]Player, 0, len(entries))
	for _, e := range entries {
		if name, ok := names[e.ID]; ok {
			out = append(out, Player{ID: e.ID, Username: name, Trophies: e.Trophies})
		}
	}
	return out, nil
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
		resp = activeCaseResp{CaseID: &active.CaseID, StoryID: &active.StoryID, StoryTitle: &active.Title}
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
