package app

import (
	"context"
	"errors"
	"net/http"
	"testing"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
)

// fakeAlertConfigRepo is a minimal in-memory AlertConfigRepository for tests.
type fakeAlertConfigRepo struct {
	rows []reminderapp.AlertTemplateConfig
	err  error
}

func (f *fakeAlertConfigRepo) GetByTypeID(_ context.Context, typeID string) ([]reminderapp.AlertTemplateConfig, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []reminderapp.AlertTemplateConfig
	for _, r := range f.rows {
		if r.TypeID == typeID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeAlertConfigRepo) GetByTypeAndKind(_ context.Context, typeID, kind string) (*reminderapp.AlertTemplateConfig, error) {
	if f.err != nil {
		return nil, f.err
	}
	for i := range f.rows {
		if f.rows[i].TypeID == typeID && f.rows[i].AlertKind == kind {
			cp := f.rows[i]
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeAlertConfigRepo) Upsert(_ context.Context, in reminderapp.AlertTemplateConfig) error {
	if f.err != nil {
		return f.err
	}
	for i := range f.rows {
		if f.rows[i].TypeID == in.TypeID && f.rows[i].AlertKind == in.AlertKind {
			f.rows[i] = in
			return nil
		}
	}
	f.rows = append(f.rows, in)
	return nil
}

var _ reminderapp.AlertConfigRepository = (*fakeAlertConfigRepo)(nil)

// fakeRegistry resolves only keys in its allow-list.
type fakeRegistry struct {
	validKeys map[string]struct{}
}

func (f *fakeRegistry) Resolve(_ context.Context, key, _ string) (notificationapp.ResolvedTemplate, error) {
	if _, ok := f.validKeys[key]; ok {
		return notificationapp.ResolvedTemplate{Key: key, Locale: "vi"}, nil
	}
	return notificationapp.ResolvedTemplate{}, errors.New("template not found: " + key)
}

var _ notificationapp.TemplateRegistry = (*fakeRegistry)(nil)

func newTestSvc(repo reminderapp.AlertConfigRepository, registry notificationapp.TemplateRegistry) AlertConfigService {
	return NewAlertConfigService(repo, registry, nil) // db=nil → skip typeID DB check
}

// GET tests

func TestGetAlertConfig_NoRows_ReturnsDefault(t *testing.T) {
	svc := newTestSvc(&fakeAlertConfigRepo{}, nil)
	dto, err := svc.GetAlertConfig(context.Background(), "dt-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.TypeID != "dt-test" {
		t.Errorf("TypeID = %q, want %q", dto.TypeID, "dt-test")
	}
	if !dto.Deadline.Enabled || dto.Deadline.TemplateKey != DefaultDeadlineAlertTemplateKey {
		t.Errorf("expected default-ON deadline config, got %+v", dto.Deadline)
	}
	if !dto.WorkflowStep.Enabled || dto.WorkflowStep.TemplateKey != DefaultWorkflowStepAlertTemplateKey {
		t.Errorf("expected default-ON workflowStep config, got %+v", dto.WorkflowStep)
	}
}

func TestGetAlertConfig_HasRows_ReturnsBothKinds(t *testing.T) {
	repo := &fakeAlertConfigRepo{rows: []reminderapp.AlertTemplateConfig{
		{TypeID: "dt-x", AlertKind: reminderapp.AlertKindDeadline, TemplateKey: "reminder.deadline_approaching", Enabled: true},
		{TypeID: "dt-x", AlertKind: reminderapp.AlertKindWorkflowStep, TemplateKey: "reminder.workflow_step_due", Enabled: false},
	}}
	svc := newTestSvc(repo, nil)
	dto, err := svc.GetAlertConfig(context.Background(), "dt-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dto.Deadline.Enabled || dto.Deadline.TemplateKey != "reminder.deadline_approaching" {
		t.Errorf("unexpected deadline: %+v", dto.Deadline)
	}
	if dto.WorkflowStep.Enabled || dto.WorkflowStep.TemplateKey != "reminder.workflow_step_due" {
		t.Errorf("unexpected workflowStep: %+v", dto.WorkflowStep)
	}
}

// PUT tests

func TestUpsertAlertConfig_HappyPath(t *testing.T) {
	repo := &fakeAlertConfigRepo{}
	registry := &fakeRegistry{validKeys: map[string]struct{}{
		"reminder.deadline_approaching": {},
		"reminder.workflow_step_due":    {},
	}}
	svc := newTestSvc(repo, registry)

	err := svc.UpsertAlertConfig(context.Background(), UpsertAlertConfigRequest{
		TypeID:  "dt-periodic",
		ActorID: "admin",
		Deadline:     AlertKindConfigInput{Enabled: true, TemplateKey: "reminder.deadline_approaching"},
		WorkflowStep: AlertKindConfigInput{Enabled: true, TemplateKey: "reminder.workflow_step_due"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify both kinds were saved.
	rows, _ := repo.GetByTypeID(context.Background(), "dt-periodic")
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
}

func TestUpsertAlertConfig_InvalidTemplateKey_FallsBackToEmptyOn(t *testing.T) {
	repo := &fakeAlertConfigRepo{}
	registry := &fakeRegistry{validKeys: map[string]struct{}{}} // nothing valid
	svc := newTestSvc(repo, registry)

	// Platform default-ON path: invalid key soft-falls back to enabled + empty key.
	err := svc.UpsertAlertConfig(context.Background(), UpsertAlertConfigRequest{
		TypeID:       "dt-x",
		Deadline:     AlertKindConfigInput{Enabled: true, TemplateKey: "nonexistent.key"},
		WorkflowStep: AlertKindConfigInput{Enabled: true, TemplateKey: ""},
	})
	if err != nil {
		t.Fatalf("expected soft fallback, got error: %v", err)
	}
	cfg, err := repo.GetByTypeAndKind(context.Background(), "dt-x", reminderapp.AlertKindDeadline)
	if err != nil || cfg == nil {
		t.Fatalf("expected deadline row, err=%v cfg=%v", err, cfg)
	}
	if !cfg.Enabled || cfg.TemplateKey != "" {
		t.Fatalf("expected enabled empty-key fallback, got %+v", cfg)
	}
}

func TestUpsertAlertConfig_DisabledWithEmptyKey_Allowed(t *testing.T) {
	repo := &fakeAlertConfigRepo{}
	registry := &fakeRegistry{validKeys: map[string]struct{}{}}
	svc := newTestSvc(repo, registry)

	// Request OFF is overridden to ON; empty registry falls back to enabled+empty key.
	err := svc.UpsertAlertConfig(context.Background(), UpsertAlertConfigRequest{
		TypeID:       "dt-x",
		Deadline:     AlertKindConfigInput{Enabled: false, TemplateKey: ""},
		WorkflowStep: AlertKindConfigInput{Enabled: false, TemplateKey: ""},
	})
	if err != nil {
		t.Fatalf("expected no error when forcing ON with empty registry, got: %v", err)
	}
	rows, _ := repo.GetByTypeID(context.Background(), "dt-x")
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	for _, r := range rows {
		if !r.Enabled {
			t.Fatalf("expected enabled=true after platform override, got %+v", r)
		}
	}
}

func TestUpsertAlertConfig_ForcesEnabledOn(t *testing.T) {
	repo := &fakeAlertConfigRepo{}
	registry := &fakeRegistry{validKeys: map[string]struct{}{
		"reminder.deadline_approaching": {},
		"reminder.workflow_step_due":    {},
	}}
	svc := newTestSvc(repo, registry)
	err := svc.UpsertAlertConfig(context.Background(), UpsertAlertConfigRequest{
		TypeID:       "dt-force-on",
		Deadline:     AlertKindConfigInput{Enabled: false, TemplateKey: "reminder.deadline_approaching"},
		WorkflowStep: AlertKindConfigInput{Enabled: false, TemplateKey: "reminder.workflow_step_due"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rows, _ := repo.GetByTypeID(context.Background(), "dt-force-on")
	for _, r := range rows {
		if !r.Enabled {
			t.Fatalf("expected forced ON, got %+v", r)
		}
	}
}

func TestUpsertAlertConfig_EmptyTypeID_Returns400(t *testing.T) {
	svc := newTestSvc(&fakeAlertConfigRepo{}, nil)
	err := svc.UpsertAlertConfig(context.Background(), UpsertAlertConfigRequest{TypeID: ""})
	if err == nil {
		t.Fatal("expected error for empty typeID")
	}
	var he *perr.HTTPError
	if !errors.As(err, &he) {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if he.HTTPStatus != http.StatusBadRequest {
		t.Errorf("HTTP status = %d, want 400", he.HTTPStatus)
	}
}
