package inmemory

import (
	"context"
	"time"

	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	disclosureinmem "github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

type Repository struct {
	disclosure *disclosureinmem.Repository
}

func NewRepository(disclosure *disclosureinmem.Repository) *Repository {
	return &Repository{disclosure: disclosure}
}

func (r *Repository) ListRows(_ context.Context, _ string, _ deadlinealertsapp.DeadlineAlertAccessScope) ([]deadlinealertsapp.AlertRow, error) {
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

func (r *Repository) GetWorkflowInstanceByRecord(_ context.Context, _, _ string) (*deadlinealertsapp.WorkflowInstanceRow, error) {
	return nil, nil
}

func (r *Repository) GetEffectiveWorkflowSnapshot(_ context.Context, _, _ string) ([]workflowapp.StepSnapshot, error) {
	return nil, nil
}

func (r *Repository) ListStepStates(_ context.Context, _ string) (map[string]deadlinealertsapp.StepRuntimeState, error) {
	return map[string]deadlinealertsapp.StepRuntimeState{}, nil
}

func (r *Repository) UpsertStepCompleted(_ context.Context, _, _, _, _ string, _ time.Time) error {
	return nil
}

func (r *Repository) UpsertStepIncomplete(_ context.Context, _, _, _, _, _ string, _ int, _ time.Time) error {
	return nil
}
