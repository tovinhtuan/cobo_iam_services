package inmemory

import (
	"context"
	"time"

	"github.com/cobo/cobo_iam_services/internal/platform/outbox"
)

// SeedBootstrapEvents inserts demo events so worker skeleton has workload.
func SeedBootstrapEvents(ctx context.Context, repo outbox.Repository) error {
	now := time.Now().UTC()
	return repo.Insert(ctx, outbox.InsertParams{
		EventID:       "evt_bootstrap_001",
		AggregateType: "system",
		AggregateID:   "bootstrap",
		EventType:     "notification.dispatch",
		PayloadJSON:   []byte(`{"message":"bootstrap dispatch"}`),
		AvailableAt:   now,
	})
}
