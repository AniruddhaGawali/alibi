// Package email is the contract between the API server (which publishes email
// jobs to a Redis queue) and the email worker (which consumes them and sends
// the mail through Brevo SMTP).
package email

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const (
	// Topic is the Redis list the API publishes to and the worker consumes.
	Topic = "queue:email:outbound"
	// DLQ receives messages that failed permanently or ran out of retries.
	DLQ = "queue:email:dlq"

	// DLQMaxLen caps the dead-letter list; the oldest entries are dropped first.
	DLQMaxLen = 1000

	TypeSignup = "signup"
)

// Message is one email job. It carries only what the worker needs to render
// the mail; the OTP is also what the user types back, so it is a short-lived
// secret: never log a Message.
type Message struct {
	To   string `json:"to"`
	OTP  string `json:"otp"`
	Type string `json:"type"`
	// ExpiresAt (unix seconds) is when the code stops being valid. The worker
	// drops a message that is still queued after that, instead of mailing a
	// dead code (e.g. after an SMTP outage). Zero means "never".
	ExpiresAt int64 `json:"expires_at,omitempty"`
}

// Publisher is the part of the queue broker the API needs (queue.Broker).
type Publisher interface {
	Publish(ctx context.Context, topic string, payload []byte) error
}

// Publish serializes m and pushes it onto the outbound email queue.
func Publish(ctx context.Context, p Publisher, m Message) error {
	payload, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("email: encode message: %w", err)
	}
	return p.Publish(ctx, Topic, payload)
}

// Expired reports whether payload is a Message whose code is already dead.
func Expired(payload []byte, now time.Time) bool {
	var m Message
	return json.Unmarshal(payload, &m) == nil && m.ExpiresAt > 0 && now.Unix() > m.ExpiresAt
}
