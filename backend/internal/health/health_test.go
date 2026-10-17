package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func call(t *testing.T, checks map[string]Check) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	New(checks).Handler(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return rec.Code, body
}

func TestAllHealthy(t *testing.T) {
	ok := func(context.Context) error { return nil }
	code, body := call(t, map[string]Check{"postgres": ok, "redis": ok})
	if code != 200 || body["status"] != "success" {
		t.Fatalf("code=%d body=%v", code, body)
	}
}

func TestDegradedReturns503WithoutLeakingErrors(t *testing.T) {
	code, body := call(t, map[string]Check{
		"postgres": func(context.Context) error { return nil },
		"redis":    func(context.Context) error { return errors.New("dial tcp 10.0.0.5:6379: refused") },
	})
	if code != 503 || body["status"] != "error" {
		t.Fatalf("code=%d body=%v", code, body)
	}
	details := body["details"].(map[string]any)
	if details["redis"] != "unavailable" || details["postgres"] != "ok" {
		t.Fatalf("details=%v", details)
	}
}
