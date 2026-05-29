package inmemory

import (
	"context"
	"time"

	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	disclosureinmem "github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
)

type Repository struct {
	disclosure *disclosureinmem.Repository
}

func NewRepository(disclosure *disclosureinmem.Repository) *Repository {
	return &Repository{disclosure: disclosure}
}

func (r *Repository) ListRows(_ context.Context, _ string) ([]deadlinealertsapp.AlertRow, error) {
	return nil, nil
}

func (r *Repository) GetCompanyDeadlineContext(ctx context.Context, companyID string) (disclosureapp.CompanyDeadlineContext, error) {
	return r.disclosure.GetCompanyDeadlineContext(ctx, companyID)
}

func (r *Repository) GetCompanyTypeDeadlineContext(ctx context.Context, companyID, typeID string) (disclosureapp.CompanyDeadlineContext, error) {
	return r.disclosure.GetCompanyTypeDeadlineContext(ctx, companyID, typeID)
}

func (r *Repository) GetTypeDeadlineConfig(ctx context.Context, companyID, typeID string) (*disclosureapp.TemplateDeadlineConfig, error) {
	_ = companyID
	_, cfg, err := r.disclosure.GetActiveVersionDeadlineConfig(ctx, typeID)
	return cfg, err
}

func (r *Repository) HasDisclosureRecord(_ context.Context, _, _ string) (bool, error) {
	return true, nil
}

func (r *Repository) ConfirmDeadlineAlert(
	_ context.Context,
	_,
	_,
	_,
	_,
	_ string,
	_ time.Time,
) error {
	return nil
}
