package chat_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"backend/internal/chat"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Runs against a real MongoDB when TEST_MONGO_URI is set (for example
// mongodb://localhost:27017). It works in a throwaway alibi_test_<random>
// database and drops it at the end.
func TestMongoStore(t *testing.T) {
	uri := os.Getenv("TEST_MONGO_URI")
	if uri == "" {
		t.Skip("TEST_MONGO_URI not set")
	}
	ctx := context.Background()
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	var rnd [6]byte
	_, _ = rand.Read(rnd[:])
	db := client.Database("alibi_test_" + hex.EncodeToString(rnd[:]))
	t.Cleanup(func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	})

	s := chat.NewMongoStore(db)
	if err := s.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureIndexes(ctx); err != nil { // idempotent
		t.Fatal(err)
	}

	const u, other, c, sus = "u1", "u2", "c1", "s1"
	base := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	ev := func(id, user, suspect string, minute int) chat.Event {
		return chat.Event{EventID: id, UserID: user, CaseID: c, SuspectID: suspect, UserQuery: "q" + id, AIResponse: "a" + id,
			Timestamp: base.Add(time.Duration(minute) * time.Minute)}
	}
	// Inserted out of order; history must come back chronological.
	for _, e := range []chat.Event{ev("3", u, sus, 3), ev("1", u, sus, 1), ev("2", u, sus, 2), ev("x", other, sus, 0), ev("y", u, "s2", 0)} {
		if err := s.Insert(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Insert(ctx, ev("1", u, sus, 1)); err != nil { // redelivery is a no-op
		t.Fatalf("duplicate: %v", err)
	}

	got, err := s.History(ctx, u, c, sus, 10)
	if err != nil || len(got) != 3 || got[0].ID != "1" || got[1].ID != "2" || got[2].ID != "3" {
		t.Fatalf("history: %+v %v", got, err)
	}
	if got[0].UserQuery != "q1" || got[0].AIResponse != "a1" || !got[0].Timestamp.Equal(base.Add(time.Minute)) {
		t.Fatalf("fields: %+v", got[0])
	}
	// SuspectOf: only the owner, in the right case.
	if sid, found, err := s.SuspectOf(ctx, u, c, "2"); err != nil || !found || sid != sus {
		t.Fatalf("suspect of own entry: %q %v %v", sid, found, err)
	}
	for _, miss := range [][3]string{{other, c, "2"}, {u, "other-case", "2"}, {u, c, "missing"}} {
		if _, found, err := s.SuspectOf(ctx, miss[0], miss[1], miss[2]); err != nil || found {
			t.Fatalf("SuspectOf%v should not find it: %v %v", miss, found, err)
		}
	}
	// The limit keeps the latest messages, still oldest first.
	if got, _ = s.History(ctx, u, c, sus, 2); len(got) != 2 || got[0].ID != "2" || got[1].ID != "3" {
		t.Fatalf("limited: %+v", got)
	}
	if got, _ = s.History(ctx, other, c, "s2", 10); len(got) != 0 || got == nil {
		t.Fatalf("other user's suspect: %+v", got)
	}
}
