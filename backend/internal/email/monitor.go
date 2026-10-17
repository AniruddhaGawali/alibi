package email

import (
	"context"
	"time"

	"backend/pkg/utils"
)

// Lengther reports queue sizes (*queue.RedisListBroker).
type Lengther interface {
	Len(ctx context.Context, topic string) (int64, error)
}

// Pruner removes queued messages matching a predicate (*queue.RedisListBroker).
type Pruner interface {
	Prune(ctx context.Context, topic string, drop func(payload []byte) bool) (int, error)
}

// pruneExpired drops messages whose OTP has expired from both queues, so
// recipient addresses and dead codes do not linger in Redis (a dead-lettered or
// stuck message is useless after its expiry).
func pruneExpired(ctx context.Context, p Pruner) {
	drop := func(payload []byte) bool { return Expired(payload, time.Now()) }
	for _, topic := range []string{Topic, DLQ} {
		n, err := p.Prune(ctx, topic, drop)
		if err != nil {
			utils.LogError("email queue monitor: prune expired", "queue", topic, "err", err)
		} else if n > 0 {
			utils.LogInfo("email queue monitor: dropped expired messages", "queue", topic, "dropped", n)
		}
	}
}

// Depths are the current queue sizes.
type Depths struct {
	Pending    int64 `json:"pending"`     // waiting to be sent
	DeadLetter int64 `json:"dead_letter"` // failed for good; needs a human
}

// Snapshot reads both queue sizes.
func Snapshot(ctx context.Context, q Lengther) (Depths, error) {
	pending, err := q.Len(ctx, Topic)
	if err != nil {
		return Depths{}, err
	}
	dead, err := q.Len(ctx, DLQ)
	if err != nil {
		return Depths{}, err
	}
	return Depths{Pending: pending, DeadLetter: dead}, nil
}

// reminderEvery is how often a non-empty DLQ is re-reported when it is not growing.
const reminderEvery = 5 * time.Minute

// Monitor samples the queues every interval until ctx is cancelled. Records
// carry stable fields (queue_pending, dlq_depth) and reach ClickHouse through
// the normal log pipeline, so alerts can be built on them. A DLQ that grew is
// logged at ERROR immediately, and a non-empty one is re-logged as a reminder
// every few minutes; a backlog (pending piling up) is logged at WARN. When q can
// prune (the real broker), expired messages are dropped from both queues first.
func Monitor(ctx context.Context, q Lengther, interval time.Duration) {
	var lastDLQ int64
	var lastReport time.Time
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		if p, ok := q.(Pruner); ok {
			pruneExpired(ctx, p)
		}
		d, err := Snapshot(ctx, q)
		switch {
		case err != nil:
			utils.LogError("email queue monitor: read depths", "err", err)
		case d.DeadLetter > 0 && (d.DeadLetter > lastDLQ || time.Since(lastReport) >= reminderEvery):
			utils.LogError("email dead-letter queue is not empty: inspect with emailctl",
				"dlq_depth", d.DeadLetter, "queue_pending", d.Pending, "dlq", DLQ)
			lastDLQ, lastReport = d.DeadLetter, time.Now()
		case d.Pending > backlogWarn:
			utils.LogWarn("email queue backlog is growing: is the worker keeping up?",
				"queue_pending", d.Pending, "dlq_depth", d.DeadLetter)
		}
		if err == nil && d.DeadLetter < lastDLQ {
			lastDLQ = d.DeadLetter // someone drained it; report again if it grows
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

const backlogWarn = 100
