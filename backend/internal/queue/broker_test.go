package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"backend/pkg/models"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newBroker(t *testing.T, opts ...Option) (*RedisListBroker, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	opts = append([]Option{WithPopTimeout(50 * time.Millisecond), WithRetryPolicy(3, time.Millisecond, 5*time.Millisecond)}, opts...)
	return NewRedisListBroker(client, "dlq", opts...), mr
}

// payloads returns the decoded payloads stored in a Redis list.
func payloads(mr *miniredis.Miniredis, key string) []string {
	items, _ := mr.List(key)
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = string(decode(it).Payload)
	}
	return out
}

func TestConsumeEmptyReturnsErrNoMessage(t *testing.T) {
	b, _ := newBroker(t)
	if err := b.Consume(context.Background(), "jobs", nil); !errors.Is(err, ErrNoMessage) {
		t.Fatalf("got %v, want ErrNoMessage", err)
	}
}

func TestConsumeSuccess(t *testing.T) {
	b, mr := newBroker(t)
	ctx := context.Background()
	_ = b.Publish(ctx, "jobs", []byte("hello"))

	var got string
	err := b.Consume(ctx, "jobs", func(_ context.Context, m models.Message) error {
		got = string(m.Payload)
		return nil
	})
	if err != nil || got != "hello" {
		t.Fatalf("err=%v got=%q", err, got)
	}
	if mr.Exists("jobs") {
		t.Fatal("message should be consumed")
	}
}

func TestDlqErrorGoesStraightToDLQ(t *testing.T) {
	b, mr := newBroker(t)
	ctx := context.Background()
	_ = b.Publish(ctx, "jobs", []byte("bad"))

	err := b.Consume(ctx, "jobs", func(context.Context, models.Message) error {
		return &models.DlqError{Reason: "malformed"}
	})
	var dlq *models.DlqError
	if !errors.As(err, &dlq) {
		t.Fatalf("got %v, want DlqError", err)
	}
	if items := payloads(mr, "dlq"); len(items) != 1 || items[0] != "bad" {
		t.Fatalf("dlq = %v", items)
	}
	if mr.Exists("jobs") {
		t.Fatal("jobs should be empty")
	}
}

func TestGenericErrorRetriesThenDeadLetters(t *testing.T) {
	b, mr := newBroker(t) // maxAttempts = 3
	ctx := context.Background()
	_ = b.Publish(ctx, "jobs", []byte("flaky"))
	boom := errors.New("boom")
	fail := func(context.Context, models.Message) error { return boom }

	for i := 1; i <= 2; i++ {
		if err := b.Consume(ctx, "jobs", fail); !errors.Is(err, boom) {
			t.Fatalf("attempt %d: got %v, want boom", i, err)
		}
		if items, _ := mr.List("jobs"); len(items) != 1 {
			t.Fatalf("attempt %d: message should be requeued, jobs=%v", i, items)
		}
	}

	err := b.Consume(ctx, "jobs", fail)
	var dlq *models.DlqError
	if !errors.As(err, &dlq) {
		t.Fatalf("third failure: got %v, want DlqError", err)
	}
	if items := payloads(mr, "dlq"); len(items) != 1 {
		t.Fatalf("dlq = %v", items)
	}
	if mr.Exists("jobs") {
		t.Fatal("jobs should be empty after dead-lettering")
	}
}

func TestWorkerErrorRequeuesWithNewPayloadAndKeepsCount(t *testing.T) {
	b, mr := newBroker(t) // maxAttempts = 3
	ctx := context.Background()
	_ = b.Publish(ctx, "jobs", []byte(`{"n":0}`))

	n := 0
	handler := func(context.Context, models.Message) error {
		n++
		return &models.WorkerError{Message: "retry", Data: map[string]int{"n": n}}
	}

	for i := 0; i < 2; i++ {
		_ = b.Consume(ctx, "jobs", handler)
	}
	items := payloads(mr, "jobs")
	if len(items) != 1 || items[0] != `{"n":2}` {
		t.Fatalf("jobs = %v, want rewritten payload", items)
	}
	// Third failure: the carried-over count reaches the limit.
	err := b.Consume(ctx, "jobs", handler)
	var dlq *models.DlqError
	if !errors.As(err, &dlq) {
		t.Fatalf("got %v, want DlqError", err)
	}
	if items := payloads(mr, "dlq"); len(items) != 1 || items[0] != `{"n":3}` {
		t.Fatalf("dlq = %v", items)
	}
}

func TestRequeueSurvivesCancelledContext(t *testing.T) {
	b, mr := newBroker(t)
	ctx, cancel := context.WithCancel(context.Background())
	_ = b.Publish(ctx, "jobs", []byte("x"))

	_ = b.Consume(ctx, "jobs", func(context.Context, models.Message) error {
		cancel() // shutdown arrives mid-handler
		return errors.New("interrupted")
	})
	if items, _ := mr.List("jobs"); len(items) != 1 {
		t.Fatalf("message lost on cancellation: jobs=%v", items)
	}
}

func TestBackoffBounds(t *testing.T) {
	b := NewRedisListBroker(nil, "dlq", WithRetryPolicy(5, 100*time.Millisecond, time.Second))
	cases := map[int]time.Duration{1: 100 * time.Millisecond, 2: 200 * time.Millisecond, 3: 400 * time.Millisecond, 5: time.Second, 200: time.Second}
	for attempts, want := range cases {
		if got := b.backoff(attempts); got != want {
			t.Errorf("backoff(%d) = %v, want %v", attempts, got, want)
		}
	}
}

func TestIdenticalPayloadsHaveIndependentRetryCounts(t *testing.T) {
	b, mr := newBroker(t) // maxAttempts = 3
	ctx := context.Background()
	_ = b.Publish(ctx, "jobs", []byte("same"))
	_ = b.Publish(ctx, "jobs", []byte("same"))
	boom := errors.New("boom")

	// Fail one message twice; its twin must not inherit those failures.
	var firstID string
	fail := func(_ context.Context, m models.Message) error {
		if firstID == "" {
			firstID = m.ID
		}
		if m.ID != firstID {
			return nil
		}
		return boom
	}
	_ = b.Consume(ctx, "jobs", fail) // first message fails, requeued
	_ = b.Consume(ctx, "jobs", fail) // twin succeeds
	_ = b.Consume(ctx, "jobs", fail) // first fails again (2 of 3)
	if mr.Exists("dlq") {
		t.Fatal("message dead-lettered before reaching max attempts")
	}
	if items := payloads(mr, "jobs"); len(items) != 1 {
		t.Fatalf("jobs = %v, want the one failing message", items)
	}
}
