package events

import "time"

// Event is a canonical outbox-compatible domain event envelope.
type Event struct {
	EventID       string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       map[string]any
	OccurredAt    time.Time
}
