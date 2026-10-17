package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoggingPreservesResponse(t *testing.T) {
	h := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("short and stout"))
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tea", nil))

	if rec.Code != http.StatusTeapot || rec.Body.String() != "short and stout" {
		t.Fatalf("code=%d body=%q", rec.Code, rec.Body)
	}
}

func TestStatusRecorderDefaultsTo200(t *testing.T) {
	var seen int
	h := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok")) // no explicit WriteHeader
		seen = w.(*statusRecorder).status
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if seen != http.StatusOK {
		t.Fatalf("status=%d, want 200", seen)
	}
}
