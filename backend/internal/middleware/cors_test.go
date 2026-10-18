package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	h := CORS([]string{"http://localhost:5173"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	pre := httptest.NewRequest(http.MethodOptions, "/api/auth/login", nil)
	pre.Header.Set("Origin", "http://localhost:5173")
	pre.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, pre)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("preflight: code=%d headers=%v", rec.Code, rec.Header())
	}

	bad := httptest.NewRequest(http.MethodGet, "/health", nil)
	bad.Header.Set("Origin", "http://evil.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, bad)
	if rec.Code != http.StatusTeapot || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("disallowed origin: code=%d headers=%v", rec.Code, rec.Header())
	}
}
