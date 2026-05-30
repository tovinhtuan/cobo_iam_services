package registry_test

import (
	"context"
	"testing"
	"testing/fstest"

	notificationregistry "github.com/cobo/cobo_iam_services/internal/notification/infra/registry"
)

func TestEmbedRegistry_ResolveFallsBackToDefaultLocale(t *testing.T) {
	registry := notificationregistry.NewEmbedRegistry()

	resolved, err := registry.Resolve(context.Background(), "auth.password_reset.user", "en")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Locale != "vi" {
		t.Fatalf("expected locale vi, got %q", resolved.Locale)
	}
}

func TestEmbedRegistry_ResolveDeadlineApproaching(t *testing.T) {
	registry := notificationregistry.NewEmbedRegistry()

	resolved, err := registry.Resolve(context.Background(), "reminder.deadline_approaching", "vi")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Subject == "" {
		t.Error("expected non-empty Subject")
	}
	if resolved.TextBody == "" {
		t.Error("expected non-empty TextBody")
	}
	if resolved.HTMLBody == "" {
		t.Error("expected non-empty HTMLBody")
	}
	wantVars := []string{"company_name", "disclosure_title", "due_date", "portal_url"}
	if len(resolved.RequiredVars) != len(wantVars) {
		t.Errorf("RequiredVars = %v, want %v", resolved.RequiredVars, wantVars)
	}
}

func TestEmbedRegistry_ResolveWorkflowStepDue(t *testing.T) {
	registry := notificationregistry.NewEmbedRegistry()

	resolved, err := registry.Resolve(context.Background(), "reminder.workflow_step_due", "vi")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Subject == "" {
		t.Error("expected non-empty Subject")
	}
	if resolved.TextBody == "" {
		t.Error("expected non-empty TextBody")
	}
	if resolved.HTMLBody == "" {
		t.Error("expected non-empty HTMLBody")
	}
	wantVars := []string{"company_name", "disclosure_title", "due_date", "portal_url", "step_name"}
	if len(resolved.RequiredVars) != len(wantVars) {
		t.Errorf("RequiredVars = %v, want %v", resolved.RequiredVars, wantVars)
	}
}

func TestEmbedRegistry_ExistingDisclosureDeadlineUnchanged(t *testing.T) {
	registry := notificationregistry.NewEmbedRegistry()

	resolved, err := registry.Resolve(context.Background(), "reminder.disclosure_deadline", "vi")
	if err != nil {
		t.Fatalf("Resolve() error = %v; existing template must still resolve", err)
	}
	if resolved.TextBody == "" {
		t.Error("existing reminder.disclosure_deadline TextBody must not be empty")
	}
}

func TestEmbedRegistry_ResolveInvalidMetaFails(t *testing.T) {
	registry := notificationregistry.NewEmbedRegistryFS(fstest.MapFS{
		"bad/meta.yaml":      &fstest.MapFile{Data: []byte(":\n")},
		"bad/vi/subject.txt": &fstest.MapFile{Data: []byte("subject")},
		"bad/vi/body.txt":    &fstest.MapFile{Data: []byte("body")},
	})

	if _, err := registry.Resolve(context.Background(), "bad", "vi"); err == nil {
		t.Fatal("expected resolve error")
	}
}
