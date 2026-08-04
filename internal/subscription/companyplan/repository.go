package companyplan

import (
	"context"
	"errors"
	"time"
)

// ErrOverlap is returned when creating/updating would overlap an occupying window.
var ErrOverlap = errors.New("company_subscription_window_overlap")

// ErrInvalidPlan is returned for invalid code/status/window inputs.
var ErrInvalidPlan = errors.New("company_subscription_invalid")

// Reader resolves commercial company plans. No plan → (nil, nil).
// Does not apply Portal badge filtering; returns actual status when a covering row exists.
type Reader interface {
	GetEffectivePlan(ctx context.Context, companyID string, at time.Time) (*CompanyPlan, error)
	GetEffectivePlans(ctx context.Context, companyIDs []string, at time.Time) (map[string]*CompanyPlan, error)
}

// Writer supports controlled inserts (DEV fixtures / future admin). Overlap must be rejected.
type Writer interface {
	Create(ctx context.Context, plan CompanyPlan) error
	DeleteByIDs(ctx context.Context, ids []string) error
	DeleteByOrigin(ctx context.Context, origin RecordOrigin) error
}

// Repository combines read and write for MySQL implementation.
type Repository interface {
	Reader
	Writer
	ListOccupyingByCompany(ctx context.Context, companyID string) ([]CompanyPlan, error)
}
