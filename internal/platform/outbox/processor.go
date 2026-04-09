package outbox

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"
)

// Handler processes one outbox event by type.
type Handler interface {
	Handle(ctx context.Context, event QueuedEvent) error
}

// HandlerFunc lets ordinary functions act as handlers.
type HandlerFunc func(ctx context.Context, event QueuedEvent) error

func (f HandlerFunc) Handle(ctx context.Context, event QueuedEvent) error { return f(ctx, event) }

type Processor struct {
	repo      Repository
	handlers  map[string]Handler
	batchSize int
	now       func() time.Time
}

func NewProcessor(repo Repository, batchSize int) *Processor {
	if batchSize <= 0 {
		batchSize = 50
	}
	return &Processor{repo: repo, handlers: map[string]Handler{}, batchSize: batchSize, now: time.Now}
}

func (p *Processor) Register(eventType string, h Handler) {
	if eventType == "" || h == nil {
		return
	}
	p.handlers[eventType] = h
}

func (p *Processor) Tick(ctx context.Context) error {
	events, err := p.repo.LockPendingBatch(ctx, p.batchSize, p.now())
	if err != nil {
		return fmt.Errorf("lock pending batch: %w", err)
	}
	const maxOutboxRetries = 10
	for _, e := range events {
		h, ok := p.handlers[e.EventType]
		if !ok {
			_ = p.repo.MarkProcessed(ctx, e.EventID, p.now())
			continue
		}
		if err := h.Handle(ctx, e); err != nil {
			nextCount := e.RetryCount + 1
			if nextCount >= maxOutboxRetries {
				_ = p.repo.MarkFailedPermanent(ctx, e.EventID, p.now(), err.Error())
				continue
			}
			next := p.now().Add(backoffWithJitter(nextCount))
			_ = p.repo.MarkRetry(ctx, e.EventID, nextCount, next, err.Error())
			continue
		}
		_ = p.repo.MarkProcessed(ctx, e.EventID, p.now())
	}
	return nil
}

func backoff(retry int) time.Duration {
	if retry < 1 {
		retry = 1
	}
	if retry > 6 {
		retry = 6
	}
	return time.Duration(1<<uint(retry-1)) * time.Second
}

func backoffWithJitter(retry int) time.Duration {
	base := backoff(retry)
	if base <= 0 {
		return time.Second
	}
	// Up to ~25% extra delay to spread retries.
	maxJitter := max(int64(base/4), 1)
	return base + time.Duration(rand.Int64N(maxJitter+1))
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
