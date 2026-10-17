package models

import "fmt"

// Message is a unit of work delivered by a queue broker.
type Message struct {
	ID      string
	Payload []byte
}

// DlqError signals that a message should be moved to the dead-letter queue.
type DlqError struct {
	Reason string
}

func (e *DlqError) Error() string {
	return fmt.Sprintf("dead-letter: %s", e.Reason)
}

// WorkerError signals that processing failed and the message should be
// re-queued with updated Data (e.g. an incremented retry counter).
type WorkerError struct {
	Message string
	Data    any
}

func (e *WorkerError) Error() string {
	return fmt.Sprintf("worker error: %s", e.Message)
}
