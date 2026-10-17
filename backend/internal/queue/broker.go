package queue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"backend/pkg/models"
	"github.com/redis/go-redis/v9"
)

type (
	Handler func(ctx context.Context, msg models.Message) error
	Broker  interface {
		Publish(ctx context.Context, topic string, payload []byte) error
		Consume(ctx context.Context, topic string, handler Handler) error
	}
)

// ErrNoMessage is returned by Consume when the topic stayed empty for the
// whole poll window. It is not a failure: callers just poll again.
var ErrNoMessage = errors.New("queue: no message available")

const (
	defaultMaxAttempts = 5
	defaultBaseBackoff = 500 * time.Millisecond
	defaultMaxBackoff  = 30 * time.Second
	defaultPopTimeout  = 2 * time.Second
	cleanupTimeout     = 5 * time.Second
)

type RedisListBroker struct {
	client      *redis.Client
	dlqName     string
	maxAttempts int
	baseBackoff time.Duration
	maxBackoff  time.Duration
	popTimeout  time.Duration
}

// Option customizes a RedisListBroker.
type Option func(*RedisListBroker)

// WithRetryPolicy sets how many times a failing message is attempted in total
// before it is moved to the DLQ, and the exponential backoff bounds between
// attempts.
func WithRetryPolicy(maxAttempts int, base, maxBackoff time.Duration) Option {
	return func(b *RedisListBroker) {
		b.maxAttempts, b.baseBackoff, b.maxBackoff = maxAttempts, base, maxBackoff
	}
}

// WithPopTimeout sets how long Consume blocks waiting for a message.
func WithPopTimeout(d time.Duration) Option {
	return func(b *RedisListBroker) { b.popTimeout = d }
}

func NewRedisListBroker(client *redis.Client, dlqName string, opts ...Option) *RedisListBroker {
	b := &RedisListBroker{
		client:      client,
		dlqName:     dlqName,
		maxAttempts: defaultMaxAttempts,
		baseBackoff: defaultBaseBackoff,
		maxBackoff:  defaultMaxBackoff,
		popTimeout:  defaultPopTimeout,
	}
	for _, o := range opts {
		o(b)
	}
	return b
}

// envelope is what actually sits in Redis. It gives every message its own ID
// and carries the failure count with it, so retries are tracked per message
// (two publishes of identical bytes never share a counter) and nothing
// outside the message has to survive a crash.
type envelope struct {
	ID       string `json:"id"`
	Attempts int    `json:"attempts"`
	Payload  []byte `json:"payload"`
}

func newID() string {
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}

func (b *RedisListBroker) Publish(ctx context.Context, topic string, payload []byte) error {
	return b.push(ctx, topic, envelope{ID: newID(), Payload: payload})
}

func (b *RedisListBroker) push(ctx context.Context, topic string, env envelope) error {
	raw, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("queue: encode message: %w", err)
	}
	return b.client.LPush(ctx, topic, raw).Err()
}

// decode reads an envelope. Anything that is not one (e.g. pushed by hand)
// is treated as a fresh message with that payload.
func decode(raw string) envelope {
	var env envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil || env.ID == "" {
		return envelope{ID: newID(), Payload: []byte(raw)}
	}
	return env
}

// Consume pops one message and runs handler on it.
//
//   - success: nothing more to do.
//   - *models.DlqError: the message goes straight to the DLQ.
//   - *models.WorkerError: the message is re-queued with wErr.Data as its new payload.
//   - any other error: the message is re-queued unchanged.
//
// Re-queued messages are counted; after maxAttempts failures they go to the
// DLQ instead, so a poison message cannot loop forever. The message is put
// back on the topic before the backoff wait, so a crash during the wait
// cannot lose it; the wait then throttles this consumer between attempts to
// avoid a hot retry loop.
func (b *RedisListBroker) Consume(ctx context.Context, topic string, handler Handler) error {
	data, err := b.client.BRPop(ctx, b.popTimeout, topic).Result()
	if errors.Is(err, redis.Nil) {
		return ErrNoMessage
	}
	if err != nil {
		return fmt.Errorf("queue: pop %q: %w", topic, err)
	}
	env := decode(data[1])

	herr := handler(ctx, models.Message{ID: env.ID, Payload: env.Payload})
	if herr == nil {
		return nil
	}

	// Cleanup must still run if ctx was cancelled mid-handler, otherwise the
	// message would be lost.
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()

	var dlqErr *models.DlqError
	if errors.As(herr, &dlqErr) {
		return b.deadLetter(cctx, topic, env, dlqErr)
	}

	var wErr *models.WorkerError
	if errors.As(herr, &wErr) && wErr.Data != nil {
		payload, mErr := json.Marshal(wErr.Data)
		if mErr != nil {
			// Requeue the original rather than dropping the message.
			slog.ErrorContext(ctx, "queue: marshal worker error data", "topic", topic, "err", mErr)
			herr = errors.Join(herr, mErr)
		} else {
			env.Payload = payload
		}
	}

	env.Attempts++
	if env.Attempts >= b.maxAttempts {
		reason := fmt.Sprintf("max attempts (%d) exceeded: %v", b.maxAttempts, herr)
		return b.deadLetter(cctx, topic, env, &models.DlqError{Reason: reason})
	}

	if pErr := b.push(cctx, topic, env); pErr != nil {
		slog.ErrorContext(ctx, "queue: requeue failed, message lost", "topic", topic, "err", pErr)
		return errors.Join(herr, pErr)
	}
	b.sleep(ctx, b.backoff(env.Attempts))
	return herr
}

// deadLetter moves the message to the DLQ. If that fails the message goes back
// on its topic so it is not lost.
func (b *RedisListBroker) deadLetter(ctx context.Context, topic string, env envelope, dlqErr *models.DlqError) error {
	slog.ErrorContext(ctx, "queue: message dead-lettered", "topic", topic, "dlq", b.dlqName, "reason", dlqErr.Reason)
	if err := b.push(ctx, b.dlqName, env); err != nil {
		slog.ErrorContext(ctx, "queue: DLQ publish failed, requeueing", "topic", topic, "err", err)
		if rErr := b.push(ctx, topic, env); rErr != nil {
			return errors.Join(dlqErr, err, rErr)
		}
		return errors.Join(dlqErr, err)
	}
	return dlqErr
}

func (b *RedisListBroker) backoff(attempts int) time.Duration {
	d := b.baseBackoff << max(attempts-1, 0)
	if d <= 0 || d > b.maxBackoff { // <= 0 guards shift overflow
		return b.maxBackoff
	}
	return d
}

// sleep waits for d or until ctx is cancelled.
func (b *RedisListBroker) sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
	}
}
