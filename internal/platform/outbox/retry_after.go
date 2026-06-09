package outbox

import "time"

// RetryScheduler is implemented by handler errors that need to dictate WHEN the
// event becomes eligible for redelivery, overriding the processor's default
// exponential backoff.
//
// The outbox processor probes every handler error with errors.As; any error in
// the chain that exposes a non-zero RetryAt() pins the next available_at to that
// instant. This lets an application keep its own retry schedule authoritative
// (e.g. the email pipeline's 1m/5m/15m/1h/6h budget) without the application
// package importing this one — the interface is satisfied structurally, so there
// is no new import edge and no Batch 2A architecture change.
//
// Returning an error WITHOUT implementing this interface preserves the legacy
// behaviour (processor-owned exponential backoff with jitter).
type RetryScheduler interface {
	error
	// RetryAt returns the absolute UTC time the event should next be attempted.
	// A zero value means "no opinion" and the processor falls back to its own
	// backoff.
	RetryAt() time.Time
}
