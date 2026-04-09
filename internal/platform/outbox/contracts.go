package outbox

import (
	"context"
	"time"

	"github.com/cobo/cobo_iam_services/internal/platform/events"
)

// Publisher appends events to transactional outbox.
type Publisher interface {
	Publish(ctx context.Context, event events.Event) error
}

// Repository is the storage port for outbox events.
type Repository interface {
	Insert(ctx context.Context, p InsertParams) error
	LockPendingBatch(ctx context.Context, batchSize int, now time.Time) ([]QueuedEvent, error)
	MarkProcessed(ctx context.Context, eventID string, processedAt time.Time) error
	MarkRetry(ctx context.Context, eventID string, retryCount int, nextAt time.Time, lastErr string) error
	// MarkFailedPermanent stops retries (status failed_permanent); used after max retry budget.
	MarkFailedPermanent(ctx context.Context, eventID string, at time.Time, lastErr string) error
}

type InsertParams struct {
	EventID       string
	AggregateType string
	AggregateID   string
	EventType     string
	PayloadJSON   []byte
	AvailableAt   time.Time
}

type QueuedEvent struct {
	EventID       string
	AggregateType string
	AggregateID   string
	EventType     string
	PayloadJSON   []byte
	RetryCount    int
	AvailableAt   time.Time
}
