package chat

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"backend/pkg/models"
)

const (
	user    = "11111111-1111-1111-1111-111111111111"
	caseID  = "22222222-2222-2222-2222-222222222222"
	suspect = "33333333-3333-3333-3333-333333333333"
)

type fakeInserter struct {
	got []Event
	err error
}

func (f *fakeInserter) Insert(_ context.Context, ev Event) error {
	if f.err != nil {
		return f.err
	}
	f.got = append(f.got, ev)
	return nil
}

func event() Event {
	return Event{EventID: "e1", UserID: user, CaseID: caseID, PlayID: "p", SuspectID: suspect, SuspectName: "Thomas",
		UserQuery: "Where were you?", AIResponse: "In the pantry.", Model: "m", Timestamp: time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)}
}

func TestWorkerHandle(t *testing.T) {
	store := &fakeInserter{}
	w := NewWorker(nil, store)
	payload, _ := json.Marshal(event())
	if err := w.Handle(context.Background(), models.Message{ID: "m1", Payload: payload}); err != nil || len(store.got) != 1 || store.got[0] != event() {
		t.Fatalf("err=%v got=%+v", err, store.got)
	}

	var dlq *models.DlqError
	if err := w.Handle(context.Background(), models.Message{Payload: []byte("{nope")}); !errors.As(err, &dlq) {
		t.Fatalf("malformed: %v", err)
	}
	missing := event()
	missing.UserID = ""
	payload, _ = json.Marshal(missing)
	if err := w.Handle(context.Background(), models.Message{Payload: payload}); !errors.As(err, &dlq) {
		t.Fatalf("missing user: %v", err)
	}

	// A MongoDB failure is a plain error, so the broker retries it.
	store.err = errors.New("mongo down")
	payload, _ = json.Marshal(event())
	if err := w.Handle(context.Background(), models.Message{Payload: payload}); err == nil || errors.As(err, &dlq) {
		t.Fatalf("mongo failure: %v", err)
	}
}

type fakeReader struct {
	user, caseID, suspect string
	limit                 int
	err                   error
}

func (f *fakeReader) History(_ context.Context, u, c, s string, limit int) ([]Message, error) {
	f.user, f.caseID, f.suspect, f.limit = u, c, s, limit
	if f.err != nil {
		return nil, f.err
	}
	return []Message{{ID: "e1", CaseID: c, SuspectID: s, UserQuery: "q", AIResponse: "a"}}, nil
}

func TestHistoryHandler(t *testing.T) {
	serve := func(r *fakeReader, path string) *httptest.ResponseRecorder {
		h := NewHandler(r, func(context.Context) string { return user })
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/chat/{case_id}/{suspect_id}", h.History)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}
	r := &fakeReader{}
	rec := serve(r, "/api/chat/"+caseID+"/"+suspect)
	if rec.Code != 200 || r.user != user || r.caseID != caseID || r.suspect != suspect || r.limit != MaxHistory {
		t.Fatalf("status %d reader %+v", rec.Code, r)
	}
	var env struct{ Data []map[string]any }
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if len(env.Data) != 1 || env.Data[0]["user_query"] != "q" || env.Data[0]["ai_response"] != "a" || env.Data[0]["timestamp"] == nil {
		t.Fatalf("body %s", rec.Body)
	}
	if _, leaks := env.Data[0]["user_id"]; leaks {
		t.Fatal("response should not carry user_id")
	}
	if rec := serve(&fakeReader{}, "/api/chat/nope/"+suspect); rec.Code != 400 {
		t.Fatalf("bad id: %d", rec.Code)
	}
	if rec := serve(&fakeReader{err: errors.New("down")}, "/api/chat/"+caseID+"/"+suspect); rec.Code != 500 {
		t.Fatalf("store error: %d", rec.Code)
	}
}
