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

	"backend/internal/storage"
	"backend/pkg/utils"
	"github.com/google/uuid"
)

const (
	maxAvatarBytes  = 1 << 20  // strict limit on the uploaded image
	multipartSlack  = 64 << 10 // room for multipart boundaries and the other fields
	multipartMemory = 2 << 20
	avatarFolder    = "avatars/"
	storageTimeout  = 10 * time.Second // cleanup of replaced images in the object store

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
	images   storage.Provider             // object store for custom avatars; nil = custom uploads disabled
	decoding chan struct{}                // bounds concurrent image decodes
}

// Option customises a Handler.
type Option func(*Handler)

// WithUploadLimiter throttles custom avatar uploads per user.
func WithUploadLimiter(l UploadLimiter) Option { return func(h *Handler) { h.uploads = l } }

// WithImageStorage enables custom avatar uploads into p.
func WithImageStorage(p storage.Provider) Option { return func(h *Handler) { h.images = p } }

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
	Role         string     `json:"role"`
}

type avatarReq struct {
	Avatar int `json:"avatar" validate:"required,min=1,max=4"` // default avatars are numbered 1-4
}

func avatarOf(u *User) avatarInfo {
	switch {
	case u.AvatarURL != nil:
		return avatarInfo{Type: "custom", URL: u.AvatarURL}
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
		Avatar: avatarOf(u), Trophies: u.Trophies, TokenBalance: balance, Role: u.Role,
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
	old, err := h.store.SetAvatarDefault(r.Context(), uid, n)
	if err != nil {
		h.avatarFailed(w, r, uid, err)
		return
	}
	h.deleteImage(r.Context(), uid, old)
	utils.LogInfoCtx(r.Context(), "default avatar set", "user_id", uid, "avatar", n)
	utils.SuccessResponse(w, http.StatusOK, avatarInfo{Type: "default", ID: &n})
}

func (h *Handler) saveCustom(w http.ResponseWriter, r *http.Request, uid string, fh *multipart.FileHeader) {
	if h.images == nil {
		utils.LogWarnCtx(r.Context(), "custom avatar refused: image storage not configured")
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "custom avatars are not available right now, pick a default one", nil)
		return
	}
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
		utils.LogWarnCtx(r.Context(), "avatar decode slots busy", "user_id", uid)
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "server busy, try again shortly", nil)
		return
	}
	img, err := func() ([]byte, error) {
		defer func() { <-h.decoding }() // release the slot even if a decoder panics
		return processAvatar(data)
	}()
	if errors.Is(err, errBadImage) {
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed",
			map[string]string{"file": "must be a valid JPEG, PNG, GIF or WebP image"})
		return
	}
	if err != nil {
		utils.LogErrorCtx(r.Context(), "process avatar failed", "user_id", uid, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not save avatar", nil)
		return
	}

	// A fresh name per upload: the URL never changes content, so it can be cached forever.
	path := fmt.Sprintf("%s%s_%s.webp", avatarFolder, uid, uuid.NewString())
	url, err := h.images.UploadImage(r.Context(), path, img)
	if err != nil {
		utils.LogErrorCtx(r.Context(), "upload avatar failed", "user_id", uid, "path", path, "err", err)
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "could not save avatar, try again", nil)
		return
	}
	old, err := h.store.SetAvatarCustom(r.Context(), uid, url, path)
	if err != nil {
		h.deleteImage(r.Context(), uid, path) // not referenced by anyone
		h.avatarFailed(w, r, uid, err)
		return
	}
	h.deleteImage(r.Context(), uid, old)
	utils.LogInfoCtx(r.Context(), "custom avatar set", "user_id", uid, "bytes_in", len(data), "bytes_out", len(img), "path", path)
	utils.SuccessResponse(w, http.StatusOK, avatarInfo{Type: "custom", URL: &url})
}

// deleteImage removes a replaced avatar from the object store. It is best
// effort: a failure only leaves an unreferenced file behind, so it is logged
// and the request still succeeds. It runs even if the client has gone away.
func (h *Handler) deleteImage(ctx context.Context, uid, path string) {
	if path == "" || h.images == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), storageTimeout)
	defer cancel()
	if err := h.images.Delete(ctx, path); err != nil {
		utils.LogErrorCtx(ctx, "delete old avatar failed", "user_id", uid, "path", path, "err", err)
	}
}

func (h *Handler) avatarFailed(w http.ResponseWriter, r *http.Request, uid string, err error) {
	if errors.Is(err, ErrNotFound) {
		utils.ErrorResponse(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}
	utils.LogErrorCtx(r.Context(), "save avatar failed", "user_id", uid, "err", err)
	utils.ErrorResponse(w, http.StatusInternalServerError, "could not save avatar", nil)
}
