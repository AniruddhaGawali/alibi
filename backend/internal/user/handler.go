package user

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/pkg/utils"
)

const (
	maxAvatarBytes   = 1 << 20  // strict limit on the uploaded image
	multipartSlack   = 64 << 10 // room for multipart boundaries and the other fields
	multipartMemory  = 2 << 20
	defaultAvatarMax = 4 // default avatars are numbered 1..defaultAvatarMax
	avatarURLPrefix  = "/api/user/avatars/"

	maxConcurrentAvatars = 3 // image decodes at once; more get a 503 instead of exhausting memory
)

// Balancer reports a user's token balance (implemented by wallet.Service).
type Balancer interface {
	Balance(ctx context.Context, userID string) (int64, error)
}

type Handler struct {
	store    Store
	balances Balancer
	userID   func(context.Context) string // authenticated user from the request context (auth.UserID)
	uploads  UploadLimiter                // nil = unlimited
	decoding chan struct{}                // bounds concurrent image decodes
}

// Option customises a Handler.
type Option func(*Handler)

// WithUploadLimiter throttles custom avatar uploads per user.
func WithUploadLimiter(l UploadLimiter) Option { return func(h *Handler) { h.uploads = l } }

func NewHandler(store Store, balances Balancer, userID func(context.Context) string, opts ...Option) *Handler {
	h := &Handler{store: store, balances: balances, userID: userID, decoding: make(chan struct{}, maxConcurrentAvatars)}
	for _, o := range opts {
		o(h)
	}
	return h
}

type avatarInfo struct {
	Type string  `json:"type"` // "default", "custom" or "none"
	ID   *int    `json:"id,omitempty"`
	URL  *string `json:"url,omitempty"`
}

type profileResp struct {
	ID           string     `json:"id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	Avatar       avatarInfo `json:"avatar"`
	Trophies     int        `json:"trophies"`
	TokenBalance int64      `json:"token_balance"`
}

type avatarReq struct {
	Avatar int `json:"avatar" validate:"required,min=1,max=4"`
}

// avatarURL points at GET /api/user/avatars/{id}; the version suffix busts
// browser caches when the image is replaced.
func avatarURL(userID string, updated time.Time) string {
	return fmt.Sprintf("%s%s?v=%d", avatarURLPrefix, userID, updated.Unix())
}

func avatarOf(u *User) avatarInfo {
	switch {
	case u.HasAvatar && u.AvatarUpdatedAt != nil:
		url := avatarURL(u.ID, *u.AvatarUpdatedAt)
		return avatarInfo{Type: "custom", URL: &url}
	case u.AvatarDefault != nil:
		return avatarInfo{Type: "default", ID: u.AvatarDefault}
	}
	return avatarInfo{Type: "none"}
}

// Profile handles GET /api/user/profile (protected by auth.Middleware).
func (h *Handler) Profile(w http.ResponseWriter, r *http.Request) {
	uid := h.userID(r.Context())
	u, err := h.store.GetByID(r.Context(), uid)
	if errors.Is(err, ErrNotFound) {
		utils.ErrorResponse(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}
	if err != nil {
		utils.LogErrorCtx(r.Context(), "load profile failed", "user_id", uid, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not load profile", nil)
		return
	}
	balance, err := h.balances.Balance(r.Context(), uid)
	if err != nil {
		utils.LogErrorCtx(r.Context(), "load token balance failed", "user_id", uid, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not load profile", nil)
		return
	}
	utils.SuccessResponse(w, http.StatusOK, profileResp{
		ID: u.ID, Username: u.Username, Email: u.Email,
		Avatar: avatarOf(u), Trophies: u.Trophies, TokenBalance: balance,
	})
}

// SetAvatar handles POST /api/user/avatar (protected by auth.Middleware). It accepts
//   - multipart/form-data with either a "file" image part (max 1 MB) or an
//     "avatar" field holding a default avatar number, or
//   - application/json {"avatar": 2}.
func (h *Handler) SetAvatar(w http.ResponseWriter, r *http.Request) {
	uid := h.userID(r.Context())
	mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))

	switch mt {
	case "application/json":
		var req avatarReq
		if !utils.BindAndValidate(w, r, &req) {
			return
		}
		h.saveDefault(w, r, uid, req.Avatar)
	case "multipart/form-data":
		h.handleMultipart(w, r, uid)
	default:
		utils.ErrorResponse(w, http.StatusUnsupportedMediaType,
			"content type must be multipart/form-data or application/json", nil)
	}
}

func (h *Handler) handleMultipart(w http.ResponseWriter, r *http.Request, uid string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarBytes+multipartSlack)
	if err := r.ParseMultipartForm(multipartMemory); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			utils.ErrorResponse(w, http.StatusRequestEntityTooLarge, "image must be at most 1 MB", nil)
			return
		}
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid multipart body", nil)
		return
	}
	defer r.MultipartForm.RemoveAll()

	files := r.MultipartForm.File["file"]
	nums := r.MultipartForm.Value["avatar"]
	switch {
	case len(files) == 1 && len(nums) == 0:
		h.saveCustom(w, r, uid, files[0])
	case len(files) == 0 && len(nums) == 1:
		n, err := strconv.Atoi(strings.TrimSpace(nums[0]))
		req := avatarReq{Avatar: n}
		if err != nil {
			utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed",
				map[string]string{"avatar": "must be a number"})
			return
		}
		if errs := utils.Validate(req); errs != nil {
			utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed", errs)
			return
		}
		h.saveDefault(w, r, uid, req.Avatar)
	default:
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed",
			map[string]string{"avatar": "send exactly one of: file, avatar"})
	}
}

func (h *Handler) saveDefault(w http.ResponseWriter, r *http.Request, uid string, n int) {
	if err := h.store.SetAvatarDefault(r.Context(), uid, n); err != nil {
		h.avatarFailed(w, r, uid, err)
		return
	}
	utils.LogInfoCtx(r.Context(), "default avatar set", "user_id", uid, "avatar", n)
	utils.SuccessResponse(w, http.StatusOK, avatarInfo{Type: "default", ID: &n})
}

func (h *Handler) saveCustom(w http.ResponseWriter, r *http.Request, uid string, fh *multipart.FileHeader) {
	if fh.Size > maxAvatarBytes {
		utils.ErrorResponse(w, http.StatusRequestEntityTooLarge, "image must be at most 1 MB", nil)
		return
	}
	f, err := fh.Open()
	if err != nil {
		utils.LogErrorCtx(r.Context(), "open uploaded avatar failed", "user_id", uid, "err", err)
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid multipart body", nil)
		return
	}
	defer f.Close()
	// Read one byte past the limit so a lying Size header cannot slip through.
	data, err := io.ReadAll(io.LimitReader(f, maxAvatarBytes+1))
	if err != nil {
		utils.LogErrorCtx(r.Context(), "read uploaded avatar failed", "user_id", uid, "err", err)
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid multipart body", nil)
		return
	}
	if len(data) > maxAvatarBytes {
		utils.ErrorResponse(w, http.StatusRequestEntityTooLarge, "image must be at most 1 MB", nil)
		return
	}

	if h.uploads != nil {
		wait, err := h.uploads.Allow(r.Context(), uid)
		if err != nil {
			utils.LogErrorCtx(r.Context(), "avatar upload rate limit check failed", "user_id", uid, "err", err)
			utils.ErrorResponse(w, http.StatusServiceUnavailable, "service unavailable", nil)
			return
		}
		if wait > 0 {
			secs := int(wait.Seconds())
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			utils.LogInfoCtx(r.Context(), "avatar upload rate limited", "user_id", uid, "retry_after_sec", secs)
			utils.ErrorResponse(w, http.StatusTooManyRequests, "too many avatar uploads, try again later",
				map[string]int{"retry_after_seconds": secs})
			return
		}
	}
	// Decoding is the memory-hungry step: fail fast when other decodes are running.
	select {
	case h.decoding <- struct{}{}:
	default:
		w.Header().Set("Retry-After", "2")
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "server busy, try again shortly", nil)
		return
	}
	jpg, err := processAvatar(data)
	<-h.decoding
	if errors.Is(err, errBadImage) {
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed",
			map[string]string{"file": "must be a valid JPEG, PNG or GIF image"})
		return
	}
	if err != nil {
		utils.LogErrorCtx(r.Context(), "process avatar failed", "user_id", uid, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not save avatar", nil)
		return
	}
	updated, err := h.store.SetAvatarCustom(r.Context(), uid, jpg)
	if err != nil {
		h.avatarFailed(w, r, uid, err)
		return
	}
	url := avatarURL(uid, updated)
	utils.LogInfoCtx(r.Context(), "custom avatar set", "user_id", uid, "bytes_in", len(data), "bytes_out", len(jpg))
	utils.SuccessResponse(w, http.StatusOK, avatarInfo{Type: "custom", URL: &url})
}

func (h *Handler) avatarFailed(w http.ResponseWriter, r *http.Request, uid string, err error) {
	if errors.Is(err, ErrNotFound) {
		utils.ErrorResponse(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}
	utils.LogErrorCtx(r.Context(), "save avatar failed", "user_id", uid, "err", err)
	utils.ErrorResponse(w, http.StatusInternalServerError, "could not save avatar", nil)
}

type avatarIDReq struct {
	ID string `json:"id" validate:"required,uuid"`
}

// Avatar handles GET /api/user/avatars/{id}: the stored custom image as JPEG.
// It is public because <img> tags cannot send an Authorization header; the
// image is already shown to other players. This is the one handler that
// writes raw bytes instead of the JSON envelope; errors still use it.
func (h *Handler) Avatar(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if errs := utils.Validate(avatarIDReq{ID: id}); errs != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid user id", errs)
		return
	}
	// Look at updated_at first (no image bytes) so a conditional request can be
	// answered with 304 without reading the BYTEA.
	updated, err := h.store.AvatarUpdatedAt(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		utils.ErrorResponse(w, http.StatusNotFound, "avatar not found", nil)
		return
	}
	if err != nil {
		utils.LogErrorCtx(r.Context(), "load avatar failed", "user_id", id, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not load avatar", nil)
		return
	}
	etag := fmt.Sprintf(`"%x"`, updated.UnixNano())
	h.avatarHeaders(w, r, etag, updated)
	if notModified(r, etag, updated) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	data, updated, err := h.store.GetAvatar(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		utils.ErrorResponse(w, http.StatusNotFound, "avatar not found", nil)
		return
	}
	if err != nil {
		utils.LogErrorCtx(r.Context(), "load avatar failed", "user_id", id, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not load avatar", nil)
		return
	}
	etag = fmt.Sprintf(`"%x"`, updated.UnixNano()) // the image may have changed since the lookup
	h.avatarHeaders(w, r, etag, updated)
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}

// avatarHeaders sets the validators and cache policy. Only a URL carrying the
// current version (?v=<updated unix>, which avatarURL produces) is cached as
// immutable; the bare URL is revalidated after five minutes, so a replaced or
// removed avatar does not linger for a day.
func (h *Handler) avatarHeaders(w http.ResponseWriter, r *http.Request, etag string, updated time.Time) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("ETag", etag)
	w.Header().Set("Last-Modified", updated.UTC().Format(http.TimeFormat))
	if r.URL.Query().Get("v") == strconv.FormatInt(updated.Unix(), 10) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300, must-revalidate")
}

// notModified implements the If-None-Match / If-Modified-Since check.
func notModified(r *http.Request, etag string, updated time.Time) bool {
	if inm := r.Header.Get("If-None-Match"); inm != "" {
		for _, c := range strings.Split(inm, ",") {
			if c = strings.TrimSpace(c); c == etag || c == "*" || c == "W/"+etag {
				return true
			}
		}
		return false
	}
	if ims := r.Header.Get("If-Modified-Since"); ims != "" {
		if t, err := http.ParseTime(ims); err == nil && !updated.Truncate(time.Second).After(t) {
			return true
		}
	}
	return false
}
