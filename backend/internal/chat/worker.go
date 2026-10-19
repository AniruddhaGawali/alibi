package chat

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"backend/internal/queue"
	"backend/pkg/models"
	"backend/pkg/utils"
)

// Inserter stores an exchange; *MongoStore implements it.
type Inserter interface {
	Insert(ctx context.Context, ev Event) error
}

// Consumer is the part of the queue broker the worker needs (*queue.RedisListBroker).
type Consumer interface {
	Consume(ctx context.Context, topic string, handler queue.Handler) error
}

const insertTimeout = 10 * time.Second

// Worker is the chat storage consumer: it moves events from Topic into MongoDB.
type Worker struct {
	broker Consumer
	store  Inserter
	// onStored runs after an exchange is stored, so cached chat history can be dropped.
	onStored func(ctx context.Context, ev Event)
}

// OnStored sets a callback run after each exchange is stored (the response cache uses it
// to drop the player's cached history; the write is asynchronous, so only now is it fresh).
func (w *Worker) OnStored(f func(ctx context.Context, ev Event)) *Worker {
	w.onStored = f
	return w
}

func NewWorker(broker Consumer, store Inserter) *Worker {
	return &Worker{broker: broker, store: store}
}

// Run consumes Topic with concurrency goroutines until ctx is cancelled, then
// waits for in-flight messages. Each event is popped by exactly one consumer
// (across API replicas too); failures are retried with backoff by the broker and
// end up in DLQ after the last attempt.
func (w *Worker) Run(ctx context.Context, concurrency int) {
	concurrency = max(concurrency, 1)
	utils.LogInfo("chat storage worker started", "topic", Topic, "dlq", DLQ, "concurrency", concurrency)
	var wg sync.WaitGroup
	for range concurrency {
		wg.Go(func() { w.loop(ctx) })
	}
	wg.Wait()
	utils.LogInfo("chat storage worker stopped")
}

func (w *Worker) loop(ctx context.Context) {
	failures := 0
	for ctx.Err() == nil {
		err := w.broker.Consume(ctx, Topic, w.Handle)
		switch {
		case err == nil, errors.Is(err, queue.ErrNoMessage), ctx.Err() != nil:
			if failures > 0 && ctx.Err() == nil {
				utils.LogInfo("chat storage worker: queue reachable again", "after_failures", failures)
			}
			failures = 0
		default:
			// A failed message (already re-queued or dead-lettered) or a Redis error. Pause
			// (longer the longer it lasts) so an unreachable Redis cannot spin the loop, and
			// log the first failure and then only now and then, not once a second per consumer.
			failures++
			if failures == 1 || failures%30 == 0 {
				utils.LogError("chat storage worker: consume", "err", err, "consecutive_failures", failures)
			}
			select {
			case <-ctx.Done():
			case <-time.After(min(time.Second<<min(failures-1, 3), 8*time.Second)):
			}
		}
	}
}

// Handle stores one event. A malformed event goes straight to the DLQ; a
// MongoDB failure is returned so the broker retries it.
func (w *Worker) Handle(ctx context.Context, msg models.Message) error {
	var ev Event
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		return &models.DlqError{Reason: "malformed JSON payload"}
	}
	if ev.EventID == "" || ev.UserID == "" || ev.CaseID == "" || ev.SuspectID == "" || ev.Timestamp.IsZero() {
		return &models.DlqError{Reason: "chat event is missing required fields"}
	}
	ictx, cancel := context.WithTimeout(context.WithoutCancel(ctx), insertTimeout)
	defer cancel()
	if err := w.store.Insert(ictx, ev); err != nil {
		utils.LogErrorCtx(ctx, "store chat event failed; will retry", "event_id", ev.EventID, "err", err)
		return err
	}
	if w.onStored != nil {
		w.onStored(ctx, ev)
	}
	utils.LogInfoCtx(ctx, "chat event stored", "event_id", ev.EventID, "user_id", ev.UserID, "case_id", ev.CaseID)
	return nil
}
