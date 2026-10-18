package chat

import (
	"context"
	"time"

	"backend/pkg/utils"
)

// Lengther reports queue sizes (*queue.RedisListBroker).
type Lengther interface {
	Len(ctx context.Context, topic string) (int64, error)
}

// reminderEvery is how often a non-empty DLQ is re-reported when it is not growing.
const reminderEvery = 5 * time.Minute

// backlogWarn is the queue depth above which the storage workers are considered behind.
const backlogWarn = 100

// Monitor samples the chat queue and its dead-letter queue every interval until
// ctx is cancelled, so exchanges that could not be stored do not go unnoticed. The
// records carry stable fields (chat_pending, chat_dlq_depth) for alerts in
// ClickHouse. A DLQ that grew is logged at ERROR at once and re-logged every few
// minutes while it is not empty; a backlog is logged at WARN.
func Monitor(ctx context.Context, q Lengther, interval time.Duration) {
	var lastDLQ int64
	var lastReport time.Time
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		pending, perr := q.Len(ctx, Topic)
		dead, derr := q.Len(ctx, DLQ)
		switch {
		case perr != nil || derr != nil:
			utils.LogError("chat queue monitor: read depths", "pending_err", perr, "dlq_err", derr)
		case dead > 0 && (dead > lastDLQ || time.Since(lastReport) >= reminderEvery):
			utils.LogError("chat dead-letter queue is not empty: paid exchanges were not stored",
				"chat_dlq_depth", dead, "chat_pending", pending, "dlq", DLQ)
			lastDLQ, lastReport = dead, time.Now()
		case pending > backlogWarn:
			utils.LogWarn("chat queue backlog is growing: is the storage worker keeping up?",
				"chat_pending", pending, "chat_dlq_depth", dead)
		}
		if perr == nil && derr == nil && dead < lastDLQ {
			lastDLQ = dead // drained; report again if it grows
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
