package admin

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif" // register decoders for image.DecodeConfig
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"

	"backend/pkg/utils"
	"github.com/disintegration/imaging"
	"github.com/gen2brain/webp" // also registers the WebP decoder
	"github.com/google/uuid"
)

const (
	maxUploadBytes  = 10 << 20 // one image
	multipartSlack  = 64 << 10 // room for the form fields and part headers
	multipartMemory = 1 << 20  // parts above this spill to a temp file
	uploadQuality   = 82
	maxSourcePx     = 8192     // longest accepted side, checked on the header before decoding
	maxSourceArea   = 40 << 20 // pixels (~40 MP); guards against decompression bombs
)

// imageKinds maps an upload kind to its folder under cases/<case_id>/ and the
// longest side it is scaled down to (never up), keeping the aspect ratio.
var imageKinds = map[string]struct {
	dir  string
	side int
}{
	"thumbnail": {"thumbnail", 1280},
	"scene":     {"scenes", 1920},
	"suspect":   {"suspects", 768},
}

var errBadImage = errors.New("admin: unsupported or invalid image")

type uploadReq struct {
	Kind   string `json:"kind" validate:"required,oneof=thumbnail scene suspect"`
	CaseID string `json:"case_id" validate:"required,uuid"`
}

type uploadResp struct {
	URL  string `json:"url"`  // save it as thumbnail_url, image_url or avatar_url
	Path string `json:"path"` // object path in the bucket
}

// Upload handles POST /api/admin/upload: multipart/form-data with "kind"
// (thumbnail, scene or suspect), "case_id" and a "file" image (JPEG, PNG, GIF or
// WebP, at most 10 MB). The image is re-encoded to WebP, scaled down for its kind
// and stored under a fresh name at cases/<case_id>/<kind>/<uuid>.webp. It answers
// the public URL; nothing is saved on the case until the admin sends that URL with
// the case, suspect or scene.
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	if h.images == nil {
		utils.LogWarnCtx(r.Context(), "admin upload refused: image storage not configured")
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "image uploads are not configured", nil)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+multipartSlack)
	if err := r.ParseMultipartForm(multipartMemory); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			utils.ErrorResponse(w, http.StatusRequestEntityTooLarge, "image must be at most 10 MB", nil)
			return
		}
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid multipart body", nil)
		return
	}
	defer r.MultipartForm.RemoveAll()

	req := uploadReq{Kind: strings.TrimSpace(r.FormValue("kind")), CaseID: strings.TrimSpace(r.FormValue("case_id"))}
	if errs := utils.Validate(req); errs != nil {
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed", errs)
		return
	}
	files := r.MultipartForm.File["file"]
	if len(files) != 1 {
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed", map[string]string{"file": "send exactly one image"})
		return
	}
	if files[0].Size > maxUploadBytes {
		utils.ErrorResponse(w, http.StatusRequestEntityTooLarge, "image must be at most 10 MB", nil)
		return
	}
	found, err := h.store.CaseExists(r.Context(), req.CaseID)
	if err != nil {
		utils.LogErrorCtx(r.Context(), "case lookup for upload failed", "case_id", req.CaseID, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not upload the image", nil)
		return
	}
	if !found {
		utils.ErrorResponse(w, http.StatusNotFound, "case not found", nil)
		return
	}

	f, err := files[0].Open()
	if err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid multipart body", nil)
		return
	}
	defer f.Close()
	// Read one byte past the limit so a lying Size header cannot slip through.
	data, err := io.ReadAll(io.LimitReader(f, maxUploadBytes+1))
	if err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid multipart body", nil)
		return
	}
	if len(data) > maxUploadBytes {
		utils.ErrorResponse(w, http.StatusRequestEntityTooLarge, "image must be at most 10 MB", nil)
		return
	}
	kind := imageKinds[req.Kind]
	out, err := processImage(data, kind.side)
	if errors.Is(err, errBadImage) {
		utils.ErrorResponse(w, http.StatusUnprocessableEntity, "validation failed",
			map[string]string{"file": "must be a JPEG, PNG, GIF or WebP image up to 8192 px a side"})
		return
	}
	if err != nil {
		utils.LogErrorCtx(r.Context(), "encode upload failed", "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not upload the image", nil)
		return
	}

	path := "cases/" + req.CaseID + "/" + kind.dir + "/" + uuid.NewString() + ".webp"
	url, err := h.images.UploadImage(r.Context(), path, out)
	if err != nil {
		utils.LogErrorCtx(r.Context(), "admin image upload failed", "path", path, "err", err)
		utils.ErrorResponse(w, http.StatusBadGateway, "could not store the image, try again", nil)
		return
	}
	utils.LogInfoCtx(r.Context(), "admin image uploaded", "admin_id", h.userID(r.Context()), "case_id", req.CaseID,
		"kind", req.Kind, "path", path, "bytes", len(out))
	utils.SuccessResponse(w, http.StatusCreated, uploadResp{URL: url, Path: path})
}

// processImage validates data as a JPEG, PNG, GIF or WebP image, scales it so its
// longest side is at most side, and encodes it as WebP (the only format the object
// store accepts). Re-encoding also strips metadata and anything hidden in the file.
func processImage(data []byte, side int) ([]byte, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errBadImage
	}
	if format != "jpeg" && format != "png" && format != "gif" && format != "webp" {
		return nil, errBadImage
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxSourcePx || cfg.Height > maxSourcePx ||
		cfg.Width*cfg.Height > maxSourceArea {
		return nil, errBadImage
	}
	src, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, errBadImage
	}
	img := src
	if b := src.Bounds(); b.Dx() > side || b.Dy() > side {
		img = imaging.Fit(src, side, side, imaging.Lanczos)
	}
	var out bytes.Buffer
	if err := webp.Encode(&out, img, webp.Options{Quality: uploadQuality}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
