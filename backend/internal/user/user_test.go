package user

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	u     *User
	image []byte
	at    time.Time

	metaReads, imageReads int
}

func (f *fakeStore) Create(context.Context, string, string, string, int) (*User, error) {
	return f.u, nil
}
func (f *fakeStore) Availability(context.Context, string, string) (bool, bool, error) {
	return false, false, nil
}
func (f *fakeStore) GetByEmail(context.Context, string) (*User, error) { return f.u, nil }
func (f *fakeStore) GetByID(context.Context, string) (*User, error)    { return f.u, nil }
func (f *fakeStore) SetAvatarDefault(_ context.Context, _ string, n int) error {
	f.u.AvatarDefault, f.u.HasAvatar, f.image, f.u.AvatarUpdatedAt = &n, false, nil, nil
	return nil
}
func (f *fakeStore) SetAvatarCustom(_ context.Context, _ string, jpg []byte) (time.Time, error) {
	f.at = time.Unix(1_700_000_000, 0)
	f.image, f.u.HasAvatar, f.u.AvatarUpdatedAt, f.u.AvatarDefault = jpg, true, &f.at, nil
	return f.at, nil
}
func (f *fakeStore) GetAvatar(context.Context, string) ([]byte, time.Time, error) {
	f.imageReads++
	if f.image == nil {
		return nil, time.Time{}, ErrNotFound
	}
	return f.image, f.at, nil
}

func (f *fakeStore) AvatarUpdatedAt(context.Context, string) (time.Time, error) {
	f.metaReads++
	if f.image == nil {
		return time.Time{}, ErrNotFound
	}
	return f.at, nil
}

// fakeBalances stands in for wallet.Service.
type fakeBalances struct{ n int64 }

func (f *fakeBalances) Balance(context.Context, string) (int64, error) { return f.n, nil }

func newHandler(t *testing.T) (*Handler, *fakeStore, *fakeBalances) {
	t.Helper()
	bal := &fakeBalances{}
	st := &fakeStore{u: &User{ID: "11111111-1111-1111-1111-111111111111", Username: "sherlock", Email: "s@example.com", Trophies: 7}}
	h := NewHandler(st, bal, func(context.Context) string { return st.u.ID })
	return h, st, bal
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

func TestCustomAvatarIsResizedStoredAndServed(t *testing.T) {
	h, st, _ := newHandler(t)
	rec := httptest.NewRecorder()
	h.SetAvatar(rec, multipartReq(t, nil, pngBytes(600, 300)))
	if rec.Code != 200 || !st.u.HasAvatar || st.u.AvatarDefault != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}

	img, err := jpeg.Decode(bytes.NewReader(st.image))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != avatarSize || b.Dy() != avatarSize {
		t.Fatalf("size = %v", b)
	}

	// The profile points at the avatar endpoint.
	prof := httptest.NewRecorder()
	h.Profile(prof, httptest.NewRequest("GET", "/api/user/profile", nil))
	var out struct {
		Data profileResp `json:"data"`
	}
	_ = json.Unmarshal(prof.Body.Bytes(), &out)
	if out.Data.Avatar.Type != "custom" || out.Data.Avatar.URL == nil ||
		!strings.HasPrefix(*out.Data.Avatar.URL, avatarURLPrefix+st.u.ID+"?v=") {
		t.Fatalf("avatar = %+v", out.Data.Avatar)
	}

	// GET returns the JPEG bytes, public and cacheable.
	get := func(id string, mod func(*http.Request)) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", avatarURLPrefix+id, nil)
		req.SetPathValue("id", id)
		if mod != nil {
			mod(req)
		}
		r := httptest.NewRecorder()
		h.Avatar(r, req)
		return r
	}
	r := get(st.u.ID, nil)
	if r.Code != 200 || r.Header().Get("Content-Type") != "image/jpeg" || !bytes.Equal(r.Body.Bytes(), st.image) {
		t.Fatalf("serve: %d %s", r.Code, r.Header())
	}
	if r := get(st.u.ID, func(q *http.Request) { q.Header.Set("If-Modified-Since", st.at.UTC().Format(http.TimeFormat)) }); r.Code != 304 {
		t.Errorf("conditional get: %d", r.Code)
	}
	if r := get("not-a-uuid", nil); r.Code != 400 {
		t.Errorf("bad id: %d", r.Code)
	}

	// Choosing a default afterwards drops the stored image.
	h.SetAvatar(httptest.NewRecorder(), multipartReq(t, map[string]string{"avatar": "2"}, nil))
	if r := get(st.u.ID, nil); r.Code != 404 {
		t.Errorf("after default avatar: %d, want 404", r.Code)
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

func TestConditionalAvatarRequestDoesNotReadTheImage(t *testing.T) {
	h, st, _ := newHandler(t)
	h.SetAvatar(httptest.NewRecorder(), multipartReq(t, nil, pngBytes(300, 200)))
	get := func(mod func(*http.Request)) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", avatarURLPrefix+st.u.ID, nil)
		req.SetPathValue("id", st.u.ID)
		mod(req)
		r := httptest.NewRecorder()
		h.Avatar(r, req)
		return r
	}
	first := get(func(*http.Request) {})
	etag := first.Header().Get("ETag")
	if first.Code != 200 || etag == "" || first.Header().Get("Cache-Control") != "public, max-age=300, must-revalidate" {
		t.Fatalf("first: %d %v", first.Code, first.Header())
	}
	st.imageReads = 0
	if r := get(func(q *http.Request) { q.Header.Set("If-None-Match", etag) }); r.Code != 304 || r.Body.Len() != 0 {
		t.Fatalf("etag revalidation: %d", r.Code)
	}
	if r := get(func(q *http.Request) { q.Header.Set("If-Modified-Since", st.at.UTC().Format(http.TimeFormat)) }); r.Code != 304 {
		t.Fatalf("ims revalidation: %d", r.Code)
	}
	if st.imageReads != 0 {
		t.Fatalf("image bytes were read %d times for conditional requests", st.imageReads)
	}
	// A stale validator still gets the image.
	if r := get(func(q *http.Request) { q.Header.Set("If-None-Match", `"old"`) }); r.Code != 200 {
		t.Fatalf("stale etag: %d", r.Code)
	}
	// Only the current versioned URL is immutable.
	v := get(func(q *http.Request) { q.URL.RawQuery = "v=" + strconv.FormatInt(st.at.Unix(), 10) })
	if cc := v.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Fatalf("versioned url cache-control = %q", cc)
	}
	old := get(func(q *http.Request) { q.URL.RawQuery = "v=1" })
	if cc := old.Header().Get("Cache-Control"); strings.Contains(cc, "immutable") {
		t.Fatalf("stale version must not be immutable: %q", cc)
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
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "90" || st.image != nil {
		t.Fatalf("got %d retry-after=%q stored=%v", rec.Code, rec.Header().Get("Retry-After"), st.image != nil)
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
