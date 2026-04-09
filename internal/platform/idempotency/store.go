package idempotency

import (
	"context"
	"encoding/json"
)

// Params identifies one idempotent operation (unique per scope + key).
type Params struct {
	CompanyID   string
	Scope       string
	Key         string
	RequestHash string
}

// Result of TryReserve: either an open reservation (Complete or Abandon), a replay, or a conflict.
type Result struct {
	ReservationID string // Complete or Abandon when non-empty and not a replay/conflict

	Replay           bool
	ReplayHTTPStatus int
	ReplayBody       []byte // JSON body to write as-is

	Conflict bool // duplicate key with different request hash or in-flight row
}

// Store backs Idempotency-Key for safe retries (MySQL idempotency_keys).
type Store interface {
	TryReserve(ctx context.Context, p Params) (Result, error)
	Complete(ctx context.Context, reservationID string, responseEnvelopeJSON []byte) error
	Abandon(ctx context.Context, reservationID string) error
}

// Envelope is stored in response_json (http_status + JSON body of success response).
type Envelope struct {
	HTTPStatus int             `json:"http_status"`
	Body       json.RawMessage `json:"body"`
}
