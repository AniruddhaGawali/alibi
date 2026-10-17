package email

import (
	"context"
	"errors"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"backend/internal/queue"
	"backend/pkg/models"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type fakeSender struct {
	mu    sync.Mutex
	calls []string // recipients
	err   error
	body  string
}

func (f *fakeSender) Send(_ context.Context, to, _, text, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, to)
	f.body = text
	return f.err
}

func payload(to, otp, typ string) []byte {
	return []byte(`{"to":"` + to + `","otp":"` + otp + `","type":"` + typ + `"}`)
}

func TestHandleSendsRenderedEmail(t *testing.T) {
	s := &fakeSender{}
	err := NewWorker(nil, s).Handle(context.Background(), models.Message{Payload: payload("a@b.co", "123456", "signup")})
	if err != nil || len(s.calls) != 1 || s.calls[0] != "a@b.co" {
		t.Fatalf("err=%v calls=%v", err, s.calls)
	}
	if !strings.Contains(s.body, "123456") {
		t.Fatalf("code missing from body: %q", s.body)
	}
}

func TestHandleClassifiesFailures(t *testing.T) {
	var dlq *models.DlqError
	var werr *models.WorkerError
	cases := []struct {
		name    string
		payload []byte
		sendErr error
		wantDLQ bool
		wantErr bool
		wantRet bool
	}{
		{"malformed json", []byte("{"), nil, true, true, false},
		{"bad recipient", payload("nope", "123456", "signup"), nil, true, true, false},
		{"bad otp", payload("a@b.co", "12ab56", "signup"), nil, true, true, false},
		{"unknown type", payload("a@b.co", "123456", "spam"), nil, true, true, false},
		{"mailbox rejected (550)", payload("a@b.co", "123456", "signup"), &textproto.Error{Code: 550, Msg: "no such user"}, true, true, false},
		{"smtp busy (451)", payload("a@b.co", "123456", "signup"), &textproto.Error{Code: 451, Msg: "try later"}, false, true, true},
		{"network error", payload("a@b.co", "123456", "signup"), errors.New("i/o timeout"), false, true, true},
	}
	for _, c := range cases {
		err := NewWorker(nil, &fakeSender{err: c.sendErr}).Handle(context.Background(), models.Message{Payload: c.payload})
		if gotDLQ := errors.As(err, &dlq); gotDLQ != c.wantDLQ {
			t.Errorf("%s: dlq=%v want %v (err=%v)", c.name, gotDLQ, c.wantDLQ, err)
		}
		if gotRet := errors.As(err, &werr); gotRet != c.wantRet {
			t.Errorf("%s: retry=%v want %v (err=%v)", c.name, gotRet, c.wantRet, err)
		}
	}
}

func newBroker(t *testing.T) (*queue.RedisListBroker, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return queue.NewRedisListBroker(rdb, DLQ,
		queue.WithPopTimeout(20*time.Millisecond),
		queue.WithRetryPolicy(3, time.Millisecond, 2*time.Millisecond)), mr
}

func runUntil(t *testing.T, w *Worker, cond func() bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx, 3); close(done) }()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
	if !cond() {
		t.Fatal("condition not reached in time")
	}
}

func TestEndToEndDelivery(t *testing.T) {
	b, mr := newBroker(t)
	s := &fakeSender{}
	if err := Publish(context.Background(), b, Message{To: "a@b.co", OTP: "123456", Type: TypeSignup}); err != nil {
		t.Fatal(err)
	}
	runUntil(t, NewWorker(b, s), func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.calls) == 1 })
	if mr.Exists(Topic) || mr.Exists(DLQ) {
		t.Fatal("queue and DLQ should both be empty after a successful send")
	}
}

func TestPersistentFailureEndsInDLQ(t *testing.T) {
	b, mr := newBroker(t)
	s := &fakeSender{err: errors.New("connection refused")}
	_ = Publish(context.Background(), b, Message{To: "a@b.co", OTP: "123456", Type: TypeSignup})
	runUntil(t, NewWorker(b, s), func() bool { return mr.Exists(DLQ) })

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) != 3 { // maxAttempts in newBroker
		t.Fatalf("send attempts = %d, want 3", len(s.calls))
	}
	if mr.Exists(Topic) {
		t.Fatal("worker left the message on the main queue")
	}
}

func TestBadPayloadGoesStraightToDLQ(t *testing.T) {
	b, mr := newBroker(t)
	s := &fakeSender{}
	_ = b.Publish(context.Background(), Topic, []byte("garbage"))
	runUntil(t, NewWorker(b, s), func() bool { return mr.Exists(DLQ) })
	if len(s.calls) != 0 {
		t.Fatal("nothing should be sent for a malformed payload")
	}
}

func TestHandleDropsExpiredCodes(t *testing.T) {
	s := &fakeSender{}
	w := NewWorker(nil, s)
	w.now = func() time.Time { return time.Unix(2_000, 0) }

	stale := []byte(`{"to":"a@b.co","otp":"123456","type":"signup","expires_at":1999}`)
	if err := w.Handle(context.Background(), models.Message{Payload: stale}); err != nil || len(s.calls) != 0 {
		t.Fatalf("expired message: err=%v sent=%v", err, s.calls)
	}
	fresh := []byte(`{"to":"a@b.co","otp":"123456","type":"signup","expires_at":2001}`)
	if err := w.Handle(context.Background(), models.Message{Payload: fresh}); err != nil || len(s.calls) != 1 {
		t.Fatalf("fresh message: err=%v sent=%v", err, s.calls)
	}
}

func TestManyConsumersSendEachMessageOnce(t *testing.T) {
	b, _ := newBroker(t)
	s := &fakeSender{}
	for i := 0; i < 20; i++ {
		_ = Publish(context.Background(), b, Message{To: "u" + strconv.Itoa(i) + "@b.co", OTP: "123456", Type: TypeSignup})
	}
	runUntil(t, NewWorker(b, s), func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.calls) >= 20 })
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[string]int{}
	for _, c := range s.calls {
		seen[c]++
	}
	if len(seen) != 20 || len(s.calls) != 20 {
		t.Fatalf("sent %d messages to %d recipients, want 20/20", len(s.calls), len(seen))
	}
}

type fakeLen struct{ pending, dead int64 }

func (f fakeLen) Len(_ context.Context, topic string) (int64, error) {
	if topic == DLQ {
		return f.dead, nil
	}
	return f.pending, nil
}

func TestSnapshot(t *testing.T) {
	d, err := Snapshot(context.Background(), fakeLen{pending: 3, dead: 2})
	if err != nil || d.Pending != 3 || d.DeadLetter != 2 {
		t.Fatalf("%+v %v", d, err)
	}
}
