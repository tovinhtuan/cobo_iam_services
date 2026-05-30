package app

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
)

// AlertKindConfigDTO is the per-kind config returned by GET.
type AlertKindConfigDTO struct {
	Enabled     bool   `json:"enabled"`
	TemplateKey string `json:"templateKey"`
}

// AlertConfigDTO is the response shape for GET alert-config.
type AlertConfigDTO struct {
	TypeID       string             `json:"typeId"`
	Deadline     AlertKindConfigDTO `json:"deadline"`
	WorkflowStep AlertKindConfigDTO `json:"workflowStep"`
}

// UpsertAlertConfigRequest is the request shape for PUT alert-config.
type UpsertAlertConfigRequest struct {
	TypeID    string
	ActorID   string
	Deadline  AlertKindConfigInput
	WorkflowStep AlertKindConfigInput
}

// AlertKindConfigInput is one kind within UpsertAlertConfigRequest.
type AlertKindConfigInput struct {
	Enabled     bool
	TemplateKey string
}

// AlertConfigService manages alert template configs for the CMS.
type AlertConfigService interface {
	GetAlertConfig(ctx context.Context, typeID string) (*AlertConfigDTO, error)
	UpsertAlertConfig(ctx context.Context, req UpsertAlertConfigRequest) error
}

type alertConfigService struct {
	repo             reminderapp.AlertConfigRepository
	templateRegistry notificationapp.TemplateRegistry
	db               *sql.DB
}

// NewAlertConfigService constructs the service.
// db is used only for typeID existence validation; may be nil (validation skipped).
func NewAlertConfigService(
	repo reminderapp.AlertConfigRepository,
	templateRegistry notificationapp.TemplateRegistry,
	db *sql.DB,
) AlertConfigService {
	return &alertConfigService{repo: repo, templateRegistry: templateRegistry, db: db}
}

func (s *alertConfigService) GetAlertConfig(ctx context.Context, typeID string) (*AlertConfigDTO, error) {
	typeID = strings.TrimSpace(typeID)
	if typeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "typeId is required", nil)
	}
	rows, err := s.repo.GetByTypeID(ctx, typeID)
	if err != nil {
		return nil, err
	}
	dto := &AlertConfigDTO{
		TypeID:       typeID,
		Deadline:     AlertKindConfigDTO{},
		WorkflowStep: AlertKindConfigDTO{},
	}
	for _, r := range rows {
		switch r.AlertKind {
		case reminderapp.AlertKindDeadline:
			dto.Deadline = AlertKindConfigDTO{Enabled: r.Enabled, TemplateKey: r.TemplateKey}
		case reminderapp.AlertKindWorkflowStep:
			dto.WorkflowStep = AlertKindConfigDTO{Enabled: r.Enabled, TemplateKey: r.TemplateKey}
		}
	}
	return dto, nil
}

func (s *alertConfigService) UpsertAlertConfig(ctx context.Context, req UpsertAlertConfigRequest) error {
	typeID := strings.TrimSpace(req.TypeID)
	if typeID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "typeId is required", nil)
	}

	// Validate typeID exists in disclosure_types.
	if s.db != nil {
		var dummy int
		err := s.db.QueryRowContext(ctx, `SELECT 1 FROM disclosure_types WHERE type_id = ? LIMIT 1`, typeID).Scan(&dummy)
		if errors.Is(err, sql.ErrNoRows) {
			return perr.NewHTTPError(http.StatusNotFound, perr.CodeDisclosureTypeNotFound,
				"disclosure type not found: "+typeID, nil)
		}
		if err != nil {
			return err
		}
	}

	// Validate each enabled kind has a valid template key.
	if err := s.validateKind(ctx, req.Deadline); err != nil {
		return err
	}
	if err := s.validateKind(ctx, req.WorkflowStep); err != nil {
		return err
	}

	actorID := strings.TrimSpace(req.ActorID)
	if actorID == "" {
		actorID = "cms"
	}

	// Upsert both kinds.
	kinds := []struct {
		kind string
		in   AlertKindConfigInput
	}{
		{reminderapp.AlertKindDeadline, req.Deadline},
		{reminderapp.AlertKindWorkflowStep, req.WorkflowStep},
	}
	for _, k := range kinds {
		if err := s.repo.Upsert(ctx, reminderapp.AlertTemplateConfig{
			TypeID:      typeID,
			AlertKind:   k.kind,
			TemplateKey: strings.TrimSpace(k.in.TemplateKey),
			Enabled:     k.in.Enabled,
			CreatedBy:   actorID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *alertConfigService) validateKind(ctx context.Context, in AlertKindConfigInput) error {
	key := strings.TrimSpace(in.TemplateKey)
	if !in.Enabled || key == "" {
		return nil
	}
	if s.templateRegistry == nil {
		return nil
	}
	if _, err := s.templateRegistry.Resolve(ctx, key, "vi"); err != nil {
		return perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeTemplateKeyNotFound,
			"template key not found: "+key, nil)
	}
	return nil
}
