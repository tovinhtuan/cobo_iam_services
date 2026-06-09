package outbox

import (
	"context"
	"errors"
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

// WithClock overrides the time source used for locking and retry scheduling.
// Tests inject a controllable clock so retry timing can be asserted without
// time.Sleep. A nil clock is ignored. Returns the receiver for chaining.
func (p *Processor) WithClock(now func() time.Time) *Processor {
	if now != nil {
		p.now = now
	}
	return p
}

// RequeueStaleProcessing flips outbox rows stuck in `processing` (whose
// available_at predates olderThan) back to `pending` so a crashed/restarted
// worker's in-flight events are re-picked instead of stranded. It delegates to
// the repository; no schema change is involved (status + available_at only).
func (p *Processor) RequeueStaleProcessing(ctx context.Context, olderThan time.Time) (int, error) {
	return p.repo.RequeueStaleProcessing(ctx, olderThan)
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
			// A handler may pin the next attempt time (e.g. the email pipeline's
			// 1m/5m/15m/1h/6h budget). When present and non-zero it overrides the
			// processor's default exponential backoff so available_at reflects the
			// application's intended schedule instead of exhausting retries in
			// seconds.
			var rs RetryScheduler
			if errors.As(err, &rs) {
				if at := rs.RetryAt(); !at.IsZero() {
					next = at.UTC()
				}
			}
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
