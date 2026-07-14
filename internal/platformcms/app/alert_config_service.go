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

// Platform default template keys when CMS Alert Email UI is hidden.
const (
	DefaultDeadlineAlertTemplateKey     = "reminder.deadline_approaching"
	DefaultWorkflowStepAlertTemplateKey = "reminder.workflow_step_due"
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
		TypeID: typeID,
		Deadline: AlertKindConfigDTO{
			Enabled:     true,
			TemplateKey: DefaultDeadlineAlertTemplateKey,
		},
		WorkflowStep: AlertKindConfigDTO{
			Enabled:     true,
			TemplateKey: DefaultWorkflowStepAlertTemplateKey,
		},
	}
	hasDeadline := false
	hasWorkflow := false
	for _, r := range rows {
		switch r.AlertKind {
		case reminderapp.AlertKindDeadline:
			hasDeadline = true
			dto.Deadline = AlertKindConfigDTO{Enabled: r.Enabled, TemplateKey: r.TemplateKey}
		case reminderapp.AlertKindWorkflowStep:
			hasWorkflow = true
			dto.WorkflowStep = AlertKindConfigDTO{Enabled: r.Enabled, TemplateKey: r.TemplateKey}
		}
	}
	_ = hasDeadline
	_ = hasWorkflow
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

	// Product decision: Alert Email always ON (CMS UI hidden). Override OFF → ON.
	deadline := s.normalizeAlertKindForPlatformOn(ctx, req.Deadline, DefaultDeadlineAlertTemplateKey)
	workflow := s.normalizeAlertKindForPlatformOn(ctx, req.WorkflowStep, DefaultWorkflowStepAlertTemplateKey)

	actorID := strings.TrimSpace(req.ActorID)
	if actorID == "" {
		actorID = "cms"
	}

	// Upsert both kinds.
	kinds := []struct {
		kind string
		in   AlertKindConfigInput
	}{
		{reminderapp.AlertKindDeadline, deadline},
		{reminderapp.AlertKindWorkflowStep, workflow},
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

// normalizeAlertKindForPlatformOn forces enabled=true and fills default template key when blank.
// If a non-empty key fails registry validation, falls back to enabled=true with empty key
// (runtime still sends using fallback template code; empty key does not skip).
func (s *alertConfigService) normalizeAlertKindForPlatformOn(
	ctx context.Context,
	in AlertKindConfigInput,
	defaultKey string,
) AlertKindConfigInput {
	out := AlertKindConfigInput{
		Enabled:     true,
		TemplateKey: strings.TrimSpace(in.TemplateKey),
	}
	if out.TemplateKey == "" {
		out.TemplateKey = defaultKey
	}
	if err := s.validateKind(ctx, out); err != nil {
		return AlertKindConfigInput{Enabled: true, TemplateKey: ""}
	}
	return out
}
