package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// webpHeader is the smallest byte string IsWebP accepts.
var webpHeader = []byte("RIFF\x00\x00\x00\x00WEBPVP8 ")

func TestCheckPath(t *testing.T) {
	for _, p := range []string{"avatars/a.webp", "cases/case-01/thumbnail.webp", "a_b-c.webp"} {
		if err := CheckPath(p); err != nil {
			t.Errorf("CheckPath(%q) = %v", p, err)
		}
	}
	for _, p := range []string{"", "/abs.webp", "../x.webp", "a/../b.webp", "a//b.webp", "x.png", "x.webp.png", "a b.webp", "a?.webp"} {
		if err := CheckPath(p); !errors.Is(err, ErrInvalidPath) {
			t.Errorf("CheckPath(%q) = %v, want ErrInvalidPath", p, err)
		}
	}
}

func TestIsWebP(t *testing.T) {
	if !IsWebP(webpHeader) {
		t.Error("webp header rejected")
	}
	for _, b := range [][]byte{nil, []byte("RIFF"), {0xFF, 0xD8, 0xFF, 0xE0, 0, 0, 0, 0, 0, 0, 0, 0}, []byte("\x89PNG\r\n\x1a\n0000")} {
		if IsWebP(b) {
			t.Errorf("IsWebP(%q) = true", b)
		}
	}
}

func TestSupabaseUpload(t *testing.T) {
	var got *http.Request
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, body = r, must(io.ReadAll(r.Body))
		_, _ = io.WriteString(w, `{"Key":"alibi-assets/avatars/u_1.webp"}`)
	}))
	defer srv.Close()

	p := NewSupabaseProvider(srv.URL+"/", "service-key", "alibi-assets")
	url, err := p.UploadImage(context.Background(), "avatars/u_1.webp", webpHeader)
	if err != nil {
		t.Fatal(err)
	}
	if want := srv.URL + "/storage/v1/object/public/alibi-assets/avatars/u_1.webp"; url != want {
		t.Errorf("url = %q, want %q", url, want)
	}
	if got.Method != http.MethodPost || got.URL.Path != "/storage/v1/object/alibi-assets/avatars/u_1.webp" {
		t.Errorf("request = %s %s", got.Method, got.URL.Path)
	}
	if got.Header.Get("Authorization") != "Bearer service-key" || got.Header.Get("apikey") != "service-key" {
		t.Error("missing auth headers")
	}
	if got.Header.Get("Content-Type") != "image/webp" || got.Header.Get("x-upsert") != "true" {
		t.Errorf("headers = %v", got.Header)
	}
	if string(body) != string(webpHeader) {
		t.Error("body not forwarded")
	}
}

func TestSupabaseRejectsBeforeSending(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
	defer srv.Close()
	p := NewSupabaseProvider(srv.URL, "k", "b")

	if _, err := p.UploadImage(context.Background(), "a.webp", []byte{0xFF, 0xD8, 0xFF}); !errors.Is(err, ErrNotWebP) {
		t.Errorf("jpeg upload err = %v, want ErrNotWebP", err)
	}
	if _, err := p.UploadImage(context.Background(), "a.jpg", webpHeader); !errors.Is(err, ErrInvalidPath) {
		t.Errorf("bad path err = %v, want ErrInvalidPath", err)
	}
	if calls != 0 {
		t.Errorf("%d requests sent for invalid uploads", calls)
	}
}

func TestSupabaseErrorsAndDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/gone.webp"):
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"statusCode":"404","error":"not_found"}`)
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"error":"Unauthorized"}`)
		}
	}))
	defer srv.Close()
	p := NewSupabaseProvider(srv.URL, "bad", "b")
	ctx := context.Background()

	var se *StatusError
	if _, err := p.UploadImage(ctx, "a.webp", webpHeader); !errors.As(err, &se) || se.Code != http.StatusForbidden {
		t.Errorf("upload err = %v, want 403 StatusError", err)
	}
	if err := p.Delete(ctx, "x.webp"); err != nil {
		t.Errorf("delete: %v", err)
	}
	if err := p.Delete(ctx, "gone.webp"); err != nil {
		t.Errorf("delete missing object: %v, want nil", err)
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func TestCacheSecondsOnlyCachesUniqueNamesForever(t *testing.T) {
	for path, want := range map[string]int{
		"avatars/11111111_22222222.webp": immutableCacheSeconds,
		"avatars/default/avatar-1.webp":  mutableCacheSeconds,
		"cases/case-01/thumbnail.webp":   mutableCacheSeconds,
	} {
		if got := cacheSeconds(path); got != want {
			t.Errorf("%s: max-age %d, want %d", path, got, want)
		}
	}
}
