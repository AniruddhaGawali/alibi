package settings

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

type memStore struct{ s Settings }

func (m *memStore) Get(context.Context) (*Settings, error) { c := m.s; return &c, nil }
func (m *memStore) Seed(_ context.Context, s Settings) error {
	if m.s.ResetTime == "" {
		m.s = s
	}
	return nil
}
func (m *memStore) Update(_ context.Context, s Settings, _ string) (*Settings, error) {
	m.s = s
	return &s, nil
}

func TestMaxTokensAndRules(t *testing.T) {
	st := &memStore{}
	svc, err := New(context.Background(), st, Settings{DailyTokens: 10, MaxTokens: 10, ResetTime: "00:00", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(context.Background(), Settings{DailyTokens: 30, MaxTokens: 20, ResetTime: "00:00", Timezone: "UTC"}, ""); err != ErrDailyAboveMax {
		t.Fatalf("daily above max: %v", err)
	}
	if _, err := svc.Update(context.Background(), Settings{DailyTokens: 15, MaxTokens: 20, ResetTime: "06:00", Timezone: "UTC"}, ""); err != nil {
		t.Fatal(err)
	}
	if svc.MaxTokens(context.Background()) != 20 {
		t.Fatalf("MaxTokens = %d", svc.MaxTokens(context.Background()))
	}
	rec := httptest.NewRecorder()
	svc.PublicRules(rec, httptest.NewRequest("GET", "/api/game/rules", nil))
	var out struct{ Data map[string]any }
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if rec.Code != 200 || out.Data["daily_tokens"] != 15.0 || out.Data["max_tokens"] != 20.0 || out.Data["token_reset_time"] != "06:00" {
		t.Fatalf("rules: %d %s", rec.Code, rec.Body)
	}
	if _, leaked := out.Data["updated_at"]; leaked {
		t.Fatal("public rules carry admin fields")
	}
}
