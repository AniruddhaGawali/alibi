package user

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backend/internal/storage"
	"github.com/gen2brain/webp"
)

type fakeStore struct {
	u       *User
	path    string // object path of the custom avatar
	failSet error
	taken   string // a username another player holds
}

func (f *fakeStore) Create(context.Context, string, string, string, int) (*User, error) {
	return f.u, nil
}
func (f *fakeStore) Availability(context.Context, string, string) (bool, bool, error) {
	return false, false, nil
}
func (f *fakeStore) GetByEmail(context.Context, string) (*User, error) { return f.u, nil }
func (f *fakeStore) GetByID(context.Context, string) (*User, error)    { return f.u, nil }
func (f *fakeStore) SetAvatarDefault(_ context.Context, _ string, n int) (string, error) {
	old := f.path
	f.u.AvatarDefault, f.u.AvatarURL, f.path = &n, nil, ""
	return old, nil
}
func (f *fakeStore) SetPlayerMode(_ context.Context, _ string, mode string) error {
	f.u.PlayerMode = mode
	return nil
}
func (f *fakeStore) SetUsername(_ context.Context, _ string, name string) error {
	if strings.EqualFold(name, f.taken) {
		return ErrUsernameTaken
	}
	f.u.Username = name
	return nil
}
func (f *fakeStore) SetAvatarCustom(_ context.Context, _ string, url, path string) (string, error) {
	if f.failSet != nil {
		return "", f.failSet
	}
	old := f.path
	f.u.AvatarDefault, f.u.AvatarURL, f.path = nil, &url, path
	return old, nil
}

// fakeImages is an in-memory storage.Provider.
type fakeImages struct {
	objects   map[string][]byte
	deleted   []string
	failWrite error
}

func (f *fakeImages) UploadImage(_ context.Context, path string, data []byte) (string, error) {
	if f.failWrite != nil {
		return "", f.failWrite
	}
	if err := storage.CheckPath(path); err != nil {
		return "", err
	}
	if !storage.IsWebP(data) {
		return "", storage.ErrNotWebP
	}
	f.objects[path] = data
	return f.PublicURL(path), nil
}
func (f *fakeImages) Delete(_ context.Context, path string) error {
	delete(f.objects, path)
	f.deleted = append(f.deleted, path)
	return nil
}
func (f *fakeImages) PublicURL(path string) string { return "https://cdn.test/" + path }
func (f *fakeImages) Ping(context.Context) error   { return nil }

// fakeBalances stands in for wallet.Service.
type fakeBalances struct{ n int64 }

func (f *fakeBalances) Balance(context.Context, string) (int64, error) { return f.n, nil }

func newHandler(t *testing.T) (*Handler, *fakeStore, *fakeBalances) {
	h, st, bal, _ := newHandlerWithImages(t)
	return h, st, bal
}

func newHandlerWithImages(t *testing.T) (*Handler, *fakeStore, *fakeBalances, *fakeImages) {
	t.Helper()
	bal := &fakeBalances{}
	st := &fakeStore{u: &User{ID: "11111111-1111-1111-1111-111111111111", Username: "sherlock", Email: "s@example.com", Trophies: 7}}
	imgs := &fakeImages{objects: map[string][]byte{}}
	h := NewHandler(st, bal, func(context.Context) string { return st.u.ID }, WithImageStorage(imgs))
	return h, st, bal, imgs
}

func TestProfileReturnsTrophiesAndRedisBalance(t *testing.T) {
	h, _, bal := newHandler(t)
	bal.n = 10

	rec := httptest.NewRecorder()
	h.Profile(rec, httptest.NewRequest("GET", "/api/user/profile", nil))
	var out struct {
		Data profileResp `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if rec.Code != 200 || out.Data.Username != "sherlock" || out.Data.Trophies != 7 || out.Data.TokenBalance != 10 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if out.Data.Avatar.Type != "none" {
		t.Fatalf("avatar = %+v", out.Data.Avatar)
	}
}

func TestProfileNewUserHasZeroBalance(t *testing.T) {
	h, _, _ := newHandler(t)
	rec := httptest.NewRecorder()
	h.Profile(rec, httptest.NewRequest("GET", "/api/user/profile", nil))
	var out struct {
		Data profileResp `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Data.TokenBalance != 0 {
		t.Fatalf("balance = %d", out.Data.TokenBalance)
	}
}

func multipartReq(t *testing.T, fields map[string]string, file []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	if file != nil {
		fw, _ := mw.CreateFormFile("file", "a.png")
		_, _ = fw.Write(file)
	}
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/api/user/avatar", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func pngBytes(w, h int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestSetDefaultAvatar(t *testing.T) {
	h, st, _ := newHandler(t)
	for name, req := range map[string]*http.Request{
		"multipart": multipartReq(t, map[string]string{"avatar": "3"}, nil),
		"json": func() *http.Request {
			r := httptest.NewRequest("POST", "/api/user/avatar", bytes.NewBufferString(`{"avatar":3}`))
			r.Header.Set("Content-Type", "application/json")
			return r
		}(),
	} {
		st.u.AvatarDefault = nil
		rec := httptest.NewRecorder()
		h.SetAvatar(rec, req)
		if rec.Code != 200 || st.u.AvatarDefault == nil || *st.u.AvatarDefault != 3 {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
}

func TestSetAvatarRejectsBadInput(t *testing.T) {
	h, _, _ := newHandler(t)
	cases := map[string]struct {
		req  *http.Request
		code int
	}{
		"out of range": {multipartReq(t, map[string]string{"avatar": "5"}, nil), 422},
		"not a number": {multipartReq(t, map[string]string{"avatar": "x"}, nil), 422},
		"both":         {multipartReq(t, map[string]string{"avatar": "1"}, pngBytes(10, 10)), 422},
		"neither":      {multipartReq(t, nil, nil), 422},
		"not an image": {multipartReq(t, nil, []byte("hello, not an image")), 422},
		"over 1 MB":    {multipartReq(t, nil, bytes.Repeat([]byte{1}, 1<<20+10)), 413},
		"wrong type": {func() *http.Request {
			r := httptest.NewRequest("POST", "/", nil)
			r.Header.Set("Content-Type", "text/plain")
			return r
		}(), 415},
	}
	for name, c := range cases {
		rec := httptest.NewRecorder()
		h.SetAvatar(rec, c.req)
		if rec.Code != c.code {
			t.Errorf("%s: got %d, want %d (%s)", name, rec.Code, c.code, rec.Body)
		}
	}
}

func TestCustomAvatarIsConvertedToWebPAndUploaded(t *testing.T) {
	h, st, _, imgs := newHandlerWithImages(t)
	rec := httptest.NewRecorder()
	h.SetAvatar(rec, multipartReq(t, nil, pngBytes(600, 300)))
	if rec.Code != 200 || st.u.AvatarURL == nil || st.u.AvatarDefault != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if !strings.HasPrefix(st.path, "avatars/"+st.u.ID+"_") || !strings.HasSuffix(st.path, ".webp") {
		t.Fatalf("object path = %q", st.path)
	}
	img, err := webp.Decode(bytes.NewReader(imgs.objects[st.path]))
	if err != nil {
		t.Fatalf("stored object is not WebP: %v", err)
	}
	if b := img.Bounds(); b.Dx() != avatarSize || b.Dy() != avatarSize {
		t.Fatalf("size = %v", b)
	}

	// The profile points at the public URL in the bucket.
	prof := httptest.NewRecorder()
	h.Profile(prof, httptest.NewRequest("GET", "/api/user/profile", nil))
	var out struct {
		Data profileResp `json:"data"`
	}
	_ = json.Unmarshal(prof.Body.Bytes(), &out)
	if out.Data.Avatar.Type != "custom" || out.Data.Avatar.URL == nil || *out.Data.Avatar.URL != "https://cdn.test/"+st.path {
		t.Fatalf("avatar = %+v", out.Data.Avatar)
	}

	// Replacing it deletes the old object; choosing a default deletes the new one.
	first := st.path
	h.SetAvatar(httptest.NewRecorder(), multipartReq(t, nil, pngBytes(50, 50)))
	if st.path == first || len(imgs.deleted) != 1 || imgs.deleted[0] != first {
		t.Fatalf("replace: path %q, deleted %v", st.path, imgs.deleted)
	}
	second := st.path
	h.SetAvatar(httptest.NewRecorder(), multipartReq(t, map[string]string{"avatar": "2"}, nil))
	if st.u.AvatarURL != nil || len(imgs.objects) != 0 || imgs.deleted[1] != second {
		t.Fatalf("default: url %v, objects %v, deleted %v", st.u.AvatarURL, imgs.objects, imgs.deleted)
	}
}

func TestCustomAvatarStorageFailures(t *testing.T) {
	// Upload fails: nothing is saved.
	h, st, _, imgs := newHandlerWithImages(t)
	imgs.failWrite = errors.New("bucket down")
	rec := httptest.NewRecorder()
	h.SetAvatar(rec, multipartReq(t, nil, pngBytes(40, 40)))
	if rec.Code != http.StatusServiceUnavailable || st.u.AvatarURL != nil {
		t.Fatalf("upload failure: %d url=%v", rec.Code, st.u.AvatarURL)
	}

	// Upload works but the database write fails: the orphaned object is removed.
	h, st, _, imgs = newHandlerWithImages(t)
	st.failSet = errors.New("db down")
	rec = httptest.NewRecorder()
	h.SetAvatar(rec, multipartReq(t, nil, pngBytes(40, 40)))
	if rec.Code != http.StatusInternalServerError || len(imgs.objects) != 0 || len(imgs.deleted) != 1 {
		t.Fatalf("db failure: %d objects=%d deleted=%v", rec.Code, len(imgs.objects), imgs.deleted)
	}

	// No storage configured: custom uploads are refused, defaults still work.
	st = &fakeStore{u: &User{ID: "11111111-1111-1111-1111-111111111111"}}
	h = NewHandler(st, &fakeBalances{}, func(context.Context) string { return st.u.ID })
	rec = httptest.NewRecorder()
	h.SetAvatar(rec, multipartReq(t, nil, pngBytes(40, 40)))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("no storage: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.SetAvatar(rec, multipartReq(t, map[string]string{"avatar": "1"}, nil))
	if rec.Code != 200 {
		t.Fatalf("default without storage: %d", rec.Code)
	}
}

func TestProcessAvatarAcceptsWebPAndJPEG(t *testing.T) {
	var jpg bytes.Buffer
	_ = jpeg.Encode(&jpg, image.NewRGBA(image.Rect(0, 0, 30, 20)), nil)
	out, err := processAvatar(jpg.Bytes())
	if err != nil || !storage.IsWebP(out) {
		t.Fatalf("jpeg: %v", err)
	}
	// A WebP upload is re-encoded too.
	if again, err := processAvatar(out); err != nil || !storage.IsWebP(again) {
		t.Fatalf("webp: %v", err)
	}
}

func TestProcessAvatarRejectsHugeDimensions(t *testing.T) {
	if _, err := processAvatar(pngBytes(maxSourcePx+1, 1)); err == nil {
		t.Fatal("expected error for oversized dimensions")
	}
	// Within the side limit but over the pixel budget (a small, compressible PNG).
	if _, err := processAvatar(pngBytes(maxSourcePx, maxSourceArea/maxSourcePx+1)); err == nil {
		t.Fatal("expected error for oversized area")
	}
}

type denyUploads struct{}

func (denyUploads) Allow(context.Context, string) (time.Duration, error) {
	return 90 * time.Second, nil
}

func TestAvatarUploadRateLimited(t *testing.T) {
	h, st, _ := newHandler(t)
	WithUploadLimiter(denyUploads{})(h)
	rec := httptest.NewRecorder()
	h.SetAvatar(rec, multipartReq(t, nil, pngBytes(40, 40)))
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "90" || st.u.AvatarURL != nil {
		t.Fatalf("got %d retry-after=%q stored=%v", rec.Code, rec.Header().Get("Retry-After"), st.u.AvatarURL != nil)
	}
	// Choosing a default avatar is not an upload and stays allowed.
	rec = httptest.NewRecorder()
	h.SetAvatar(rec, multipartReq(t, map[string]string{"avatar": "2"}, nil))
	if rec.Code != 200 {
		t.Fatalf("default avatar: %d", rec.Code)
	}
}

func TestAvatarDecodeConcurrencyIsBounded(t *testing.T) {
	h, _, _ := newHandler(t)
	for i := 0; i < cap(h.decoding); i++ {
		h.decoding <- struct{}{}
	}
	rec := httptest.NewRecorder()
	h.SetAvatar(rec, multipartReq(t, nil, pngBytes(40, 40)))
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestSetModeSavesAndProfileReturnsIt(t *testing.T) {
	h, st, _ := newHandler(t)
	st.u.PlayerMode = ModeInvestigator

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/user/mode", strings.NewReader(`{"mode":"master_sleuth"}`))
	req.Header.Set("Content-Type", "application/json")
	h.SetMode(rec, req)
	if rec.Code != 200 || st.u.PlayerMode != ModeMasterSleuth {
		t.Fatalf("%d %s mode=%q", rec.Code, rec.Body, st.u.PlayerMode)
	}

	rec = httptest.NewRecorder()
	h.Profile(rec, httptest.NewRequest("GET", "/api/user/profile", nil))
	var out struct {
		Data profileResp `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Data.PlayerMode != ModeMasterSleuth {
		t.Fatalf("profile mode = %q", out.Data.PlayerMode)
	}
}

func TestSetModeRejectsUnknownMode(t *testing.T) {
	for _, body := range []string{`{"mode":"easy"}`, `{}`, `{"mode":"investigator","x":1}`} {
		h, st, _ := newHandler(t)
		st.u.PlayerMode = ModeInvestigator
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/api/user/mode", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		h.SetMode(rec, req)
		if rec.Code != 400 && rec.Code != 422 {
			t.Fatalf("%s: got %d", body, rec.Code)
		}
		if st.u.PlayerMode != ModeInvestigator {
			t.Fatalf("%s: mode changed to %q", body, st.u.PlayerMode)
		}
	}
}

func TestSetUsername(t *testing.T) {
	h, st, _ := newHandler(t)
	st.taken = "watson"
	cases := []struct {
		body string
		code int
		want string
	}{
		{`{"username":"Holmes_2"}`, 200, "Holmes_2"},
		{`{"username":"WATSON"}`, 409, "Holmes_2"},
		{`{"username":"ab"}`, 422, "Holmes_2"},
		{`{"username":"bad name"}`, 422, "Holmes_2"},
		{`{}`, 422, "Holmes_2"},
		{`{"username":"lestrade","x":1}`, 400, "Holmes_2"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/api/user/username", strings.NewReader(c.body))
		req.Header.Set("Content-Type", "application/json")
		h.SetUsername(rec, req)
		if rec.Code != c.code || st.u.Username != c.want {
			t.Fatalf("%s: got %d %s, username %q", c.body, rec.Code, rec.Body, st.u.Username)
		}
	}
}
