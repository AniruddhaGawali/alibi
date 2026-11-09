package email

import (
	"context"
	"encoding/json"
	"errors"
	"net/mail"
	"net/textproto"
	"regexp"
	"sync"
	"time"

	"backend/pkg/models"
	"backend/pkg/utils"

	"backend/internal/queue"
)

// Sender delivers one rendered message (implemented by *utils.Mailer).
type Sender interface {
	Send(ctx context.Context, to, subject, text, html string) error
}

// Broker is the part of the queue broker the worker needs (*queue.RedisListBroker).
type Broker interface {
	Consume(ctx context.Context, topic string, handler queue.Handler) error
}

var otpRe = regexp.MustCompile(`^[0-9]{6}$`)

const sendTimeout = 30 * time.Second

// Worker consumes the outbound email queue and sends each message.
type Worker struct {
	broker Broker
	sender Sender
	now    func() time.Time
}

func NewWorker(broker Broker, sender Sender) *Worker {
	return &Worker{broker: broker, sender: sender, now: time.Now}
}

// Run consumes the queue with `concurrency` parallel consumers until ctx is
// cancelled, then waits for in-flight messages. Several consumers (or several
// worker processes) are safe: each message is popped by exactly one of them. A
// message that fails is retried with backoff by the broker and, after its last
// attempt, moved to the DLQ, so one bad address can never wedge a worker.
func (w *Worker) Run(ctx context.Context, concurrency int) {
	if concurrency < 1 {
		concurrency = 1
	}
	utils.LogInfo("email worker started", "topic", Topic, "dlq", DLQ, "concurrency", concurrency)
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.loop(ctx)
		}()
	}
	wg.Wait()
	utils.LogInfo("email worker stopped")
}

func (w *Worker) loop(ctx context.Context) {
	failures := 0
	for ctx.Err() == nil {
		err := w.broker.Consume(ctx, Topic, w.Handle)
		switch {
		case err == nil, errors.Is(err, queue.ErrNoMessage), ctx.Err() != nil:
			if failures > 0 && ctx.Err() == nil {
				utils.LogInfo("email worker: queue reachable again", "after_failures", failures)
			}
			failures = 0
		default:
			// A failed message (already re-queued or dead-lettered) or a Redis error
			// (down, or a hosted plan's request limit). Pause longer the longer it lasts,
			// up to a minute, so a failing Redis is not hit (and billed) every second, and
			// log the first failure and then only now and then.
			failures++
			if failures == 1 || failures%30 == 0 {
				utils.LogError("email worker: consume", "err", err, "consecutive_failures", failures)
			}
			select {
			case <-ctx.Done():
			case <-time.After(min(time.Second<<min(failures-1, 6), time.Minute)):
			}
		}
	}
}

// Handle processes one queue message. Return values drive the broker:
//   - nil: sent.
//   - *models.DlqError: unfixable (bad payload, mailbox rejected); straight to the DLQ.
//   - *models.WorkerError: temporary (network, SMTP 4xx, rate limit); retried with backoff.
func (w *Worker) Handle(ctx context.Context, msg models.Message) error {
	var m Message
	if err := json.Unmarshal(msg.Payload, &m); err != nil {
		return &models.DlqError{Reason: "malformed JSON payload"}
	}
	if m.ExpiresAt > 0 && w.now().Unix() > m.ExpiresAt {
		// The code is already dead; sending it would only confuse the user.
		utils.LogInfoCtx(ctx, "dropping expired email", "message_id", msg.ID, "type", m.Type)
		return nil
	}
	if addr, err := mail.ParseAddress(m.To); err != nil || addr.Address != m.To {
		return &models.DlqError{Reason: "invalid recipient address"}
	}
	if !otpRe.MatchString(m.OTP) {
		return &models.DlqError{Reason: "otp must be 6 digits"}
	}
	subject, text, htmlBody, err := render(m, w.now())
	if err != nil {
		return &models.DlqError{Reason: "unknown message type"}
	}

	sctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	if err := w.sender.Send(sctx, m.To, subject, text, htmlBody); err != nil {
		var perr *textproto.Error
		if errors.Is(err, utils.ErrInvalidAddress) || (errors.As(err, &perr) && perr.Code >= 500) {
			return &models.DlqError{Reason: "permanently rejected by SMTP server: " + err.Error()}
		}
		utils.LogErrorCtx(ctx, "email send failed; will retry", "message_id", msg.ID, "type", m.Type, "err", err)
		return &models.WorkerError{Message: "smtp send failed: " + err.Error()}
	}
	utils.LogInfoCtx(ctx, "email sent", "message_id", msg.ID, "type", m.Type)
	return nil
}
