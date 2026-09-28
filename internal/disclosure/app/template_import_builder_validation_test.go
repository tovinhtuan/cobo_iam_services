package app_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
)

func TestTemplateBuilderValidation_ReusesCMSRulesWithoutTokenOrHistory(t *testing.T) {
	t.Setenv("CMS_TEMPLATE_IMPORT_HISTORY_ENABLED", "true")
	repo := inmemory.NewRepository()
	svc := newImportService(repo, []string{"platform.cms.view", "cms.template.write"})
	subject := disclosureapp.Subject{UserID: "template-builder-user", MembershipID: "membership-1", CompanyID: "cobo-platform"}

	fixturePath := filepath.Join("..", "..", "..", "docs", "schema", "template-import-v1.example.json")
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	resp, err := svc.ValidateTemplateImportForBuilder(context.Background(), disclosureapp.ValidateTemplateImportForBuilderRequest{
		Subject:   subject,
		Filename:  "builder-candidate.json",
		FileBytes: fixture,
	})
	if err != nil {
		t.Fatalf("builder validation: %v", err)
	}
	if !resp.ParseValid || !resp.DomainValid {
		t.Fatalf("expected a valid fixture, got parse_valid=%t domain_valid=%t errors=%v", resp.ParseValid, resp.DomainValid, resp.Errors)
	}
	if _, exposed := reflect.TypeOf(*resp).FieldByName("ValidationToken"); exposed {
		t.Fatal("builder response must never expose a validation token")
	}
	if _, exposed := reflect.TypeOf(*resp).FieldByName("ImportAttemptID"); exposed {
		t.Fatal("builder response must never expose an import-attempt id")
	}

	history, err := svc.ListTemplateImportHistory(context.Background(), disclosureapp.ListTemplateImportHistoryRequest{Subject: subject, Limit: 20})
	if err != nil {
		t.Fatalf("list import history: %v", err)
	}
	if len(history.Items) != 0 {
		t.Fatalf("builder validation created %d history item(s), want zero", len(history.Items))
	}
}
