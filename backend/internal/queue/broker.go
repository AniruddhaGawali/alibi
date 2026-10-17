package queue

import (
	"context"
	"crypto/sha256"
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
	attemptsTTL        = 24 * time.Hour
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

func (b *RedisListBroker) Publish(ctx context.Context, topic string, payload []byte) error {
	return b.client.LPush(ctx, topic, payload).Err()
}

// Consume pops one message and runs handler on it.
//
//   - success: nothing more to do.
//   - *models.DlqError: the message goes straight to the DLQ.
//   - *models.WorkerError: the message is re-queued with wErr.Data as its new payload.
//   - any other error: the message is re-queued unchanged.
//
// Re-queued messages are counted; after maxAttempts failures they go to the
// DLQ instead, so a poison message cannot loop forever. Between attempts the
// consumer waits with exponential backoff to avoid a hot retry loop.
func (b *RedisListBroker) Consume(ctx context.Context, topic string, handler Handler) error {
	data, err := b.client.BRPop(ctx, b.popTimeout, topic).Result()
	if errors.Is(err, redis.Nil) {
		return ErrNoMessage
	}
	if err != nil {
		return fmt.Errorf("queue: pop %q: %w", topic, err)
	}
	raw := []byte(data[1])

	herr := handler(ctx, models.Message{ID: data[0], Payload: raw})
	if herr == nil {
		b.clearAttempts(ctx, topic, raw)
		return nil
	}

	// Cleanup must still run if ctx was cancelled mid-handler, otherwise the
	// message would be lost.
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()

	var dlqErr *models.DlqError
	if errors.As(herr, &dlqErr) {
		return b.deadLetter(cctx, topic, raw, dlqErr)
	}

	next := raw
	var wErr *models.WorkerError
	if errors.As(herr, &wErr) && wErr.Data != nil {
		payload, mErr := json.Marshal(wErr.Data)
		if mErr != nil {
			// Requeue the original rather than dropping the message.
			slog.ErrorContext(ctx, "queue: marshal worker error data", "topic", topic, "err", mErr)
			herr = errors.Join(herr, mErr)
		} else {
			next = payload
		}
	}

	attempts, aErr := b.bumpAttempts(cctx, topic, raw, next)
	if aErr != nil {
		slog.ErrorContext(ctx, "queue: track attempts", "topic", topic, "err", aErr)
	}
	if attempts >= b.maxAttempts {
		reason := fmt.Sprintf("max attempts (%d) exceeded: %v", b.maxAttempts, herr)
		return b.deadLetter(cctx, topic, next, &models.DlqError{Reason: reason})
	}

	b.sleep(ctx, b.backoff(attempts))
	if pErr := b.Publish(cctx, topic, next); pErr != nil {
		slog.ErrorContext(ctx, "queue: requeue failed, message lost", "topic", topic, "err", pErr)
		return errors.Join(herr, pErr)
	}
	return herr
}

// deadLetter moves payload to the DLQ. If that fails the message goes back on
// its topic so it is not lost.
func (b *RedisListBroker) deadLetter(ctx context.Context, topic string, payload []byte, dlqErr *models.DlqError) error {
	slog.ErrorContext(ctx, "queue: message dead-lettered", "topic", topic, "dlq", b.dlqName, "reason", dlqErr.Reason)
	if err := b.Publish(ctx, b.dlqName, payload); err != nil {
		slog.ErrorContext(ctx, "queue: DLQ publish failed, requeueing", "topic", topic, "err", err)
		if rErr := b.Publish(ctx, topic, payload); rErr != nil {
			return errors.Join(dlqErr, err, rErr)
		}
		return errors.Join(dlqErr, err)
	}
	b.clearAttempts(ctx, topic, payload)
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

func attemptsKey(topic string) string { return "queue:attempts:" + topic }

func payloadID(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// bumpAttempts increments the failure count for old and, when the payload
// changed, carries the count over to next so a rewritten message keeps its
// history.
func (b *RedisListBroker) bumpAttempts(ctx context.Context, topic string, old, next []byte) (int, error) {
	key := attemptsKey(topic)
	n, err := b.client.HIncrBy(ctx, key, payloadID(old), 1).Result()
	if err != nil {
		return 1, err
	}
	pipe := b.client.Pipeline()
	if string(old) != string(next) {
		pipe.HDel(ctx, key, payloadID(old))
		pipe.HSet(ctx, key, payloadID(next), n)
	}
	pipe.Expire(ctx, key, attemptsTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return int(n), err
	}
	return int(n), nil
}

func (b *RedisListBroker) clearAttempts(ctx context.Context, topic string, payload []byte) {
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	if err := b.client.HDel(cctx, attemptsKey(topic), payloadID(payload)).Err(); err != nil {
		slog.WarnContext(ctx, "queue: clear attempts", "topic", topic, "err", err)
	}
}
