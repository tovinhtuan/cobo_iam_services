package inmemory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/cobo/cobo_iam_services/internal/platform/outbox"
)

type item struct {
	outbox.QueuedEvent
	Status      string
	LastError   string
	ProcessedAt *time.Time
}

type Repository struct {
	mu     sync.Mutex
	events map[string]*item
}

func NewRepository() *Repository {
	return &Repository{events: map[string]*item{}}
}

func (r *Repository) Insert(_ context.Context, p outbox.InsertParams) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events[p.EventID] = &item{QueuedEvent: outbox.QueuedEvent{EventID: p.EventID, AggregateType: p.AggregateType, AggregateID: p.AggregateID, EventType: p.EventType, PayloadJSON: p.PayloadJSON, RetryCount: 0, AvailableAt: p.AvailableAt}, Status: "pending"}
	return nil
}

func (r *Repository) LockPendingBatch(_ context.Context, batchSize int, now time.Time) ([]outbox.QueuedEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	all := make([]*item, 0, len(r.events))
	for _, it := range r.events {
		if it.Status == "pending" && !it.AvailableAt.After(now) {
			all = append(all, it)
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].AvailableAt.Equal(all[j].AvailableAt) {
			return all[i].EventID < all[j].EventID
		}
		return all[i].AvailableAt.Before(all[j].AvailableAt)
	})
	if batchSize > len(all) {
		batchSize = len(all)
	}
	out := make([]outbox.QueuedEvent, 0, batchSize)
	for i := 0; i < batchSize; i++ {
		all[i].Status = "processing"
		out = append(out, all[i].QueuedEvent)
	}
	return out, nil
}

func (r *Repository) MarkProcessed(_ context.Context, eventID string, processedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if it, ok := r.events[eventID]; ok {
		it.Status = "processed"
		it.ProcessedAt = &processedAt
	}
	return nil
}

func (r *Repository) MarkRetry(_ context.Context, eventID string, retryCount int, nextAt time.Time, lastErr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if it, ok := r.events[eventID]; ok {
		it.Status = "pending"
		it.RetryCount = retryCount
		it.AvailableAt = nextAt
		it.LastError = lastErr
	}
	return nil
}
