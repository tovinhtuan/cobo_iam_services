package outbox

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cobo/cobo_iam_services/internal/platform/events"
)

type PublisherImpl struct {
	repo Repository
}

func NewPublisher(repo Repository) Publisher {
	return &PublisherImpl{repo: repo}
}

func (p *PublisherImpl) Publish(ctx context.Context, event events.Event) error {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}
	return p.repo.Insert(ctx, InsertParams{
		EventID:       event.EventID,
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		EventType:     event.EventType,
		PayloadJSON:   payload,
		AvailableAt:   event.OccurredAt,
	})
}
