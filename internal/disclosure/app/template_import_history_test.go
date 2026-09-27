package app_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestImportHistoryFlagOffPreservesValidate(t *testing.T) {
	t.Setenv("CMS_TEMPLATE_IMPORT_HISTORY_ENABLED", "false")
	t.Setenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET", "test-cms-template-import-secret-for-suite")
	svc := newImportService(inmemory.NewRepository(), []string{"platform.cms.view", "cms.template.write"})
	resp, err := svc.ValidateTemplateImport(context.Background(), disclosureapp.ValidateTemplateImportRequest{
		Subject:   disclosureapp.Subject{UserID: "admin-1", MembershipID: "m1", CompanyID: "cobo-platform"},
		Filename:  "bad.json",
		FileBytes: []byte("not json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ImportAttemptID != "" {
		t.Fatalf("flag off must not return attempt id, got %s", resp.ImportAttemptID)
	}
	_, err = svc.ListTemplateImportHistory(context.Background(), disclosureapp.ListTemplateImportHistoryRequest{
		Subject: disclosureapp.Subject{UserID: "admin-1", CompanyID: "cobo-platform"},
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != 404 || he.Code != perr.CodeFeatureDisabled {
		t.Fatalf("history flag off: %+v", err)
	}
}

func TestImportHistoryFlagOnRecordsFailureAndConfirmRequiresID(t *testing.T) {
	t.Setenv("CMS_TEMPLATE_IMPORT_HISTORY_ENABLED", "true")
	t.Setenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET", "test-cms-template-import-secret-for-suite")
	repo := inmemory.NewRepository()
	svc := newImportService(repo, []string{"platform.cms.view", "cms.template.write"})
	sub := disclosureapp.Subject{UserID: "admin-1", MembershipID: "m1", CompanyID: "cobo-platform"}
	resp, err := svc.ValidateTemplateImport(context.Background(), disclosureapp.ValidateTemplateImportRequest{
		Subject:   sub,
		Filename:  `C:\tmp\wrapped.json`,
		FileBytes: []byte("hello {"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ImportAttemptID == "" || resp.ParseValid {
		t.Fatalf("expected failed attempt id, resp=%+v", resp)
	}
	page, err := svc.ListTemplateImportHistory(context.Background(), disclosureapp.ListTemplateImportHistoryRequest{Subject: sub, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Status != disclosureapp.ImportAttemptStatusValidationFailed {
		t.Fatalf("items=%+v", page.Items)
	}
	if page.Items[0].Filename != "wrapped.json" {
		t.Fatalf("filename=%s", page.Items[0].Filename)
	}
	detail, err := svc.GetTemplateImportHistory(context.Background(), disclosureapp.GetTemplateImportHistoryRequest{Subject: sub, ID: resp.ImportAttemptID})
	if err != nil {
		t.Fatal(err)
	}
	if detail.FileSHA256 == "" || len(detail.ErrorCodes) == 0 {
		t.Fatalf("detail missing hash or codes: %+v", detail)
	}
	_, err = svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
		Subject:         sub,
		TargetTypeID:    "import-history-1",
		TargetName:      "name",
		ValidationToken: "missing",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeImportAttemptRequired {
		t.Fatalf("missing id: %+v", err)
	}
	other := sub
	other.CompanyID = "other-co"
	_, err = svc.GetTemplateImportHistory(context.Background(), disclosureapp.GetTemplateImportHistoryRequest{Subject: other, ID: resp.ImportAttemptID})
	he, ok = perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeImportAttemptNotFound {
		t.Fatalf("cross company detail: %+v", err)
	}
}

func TestPeriodicEventBasedPeriodicityRejected(t *testing.T) {
	tpl := disclosureapp.TemplateImportDefinitionV1{
		Name:             "bad periodic",
		TemplateCategory: "periodic",
		Periodicity:      "event_based",
		DeadlineRule:     "T+5",
		ApplicabilityRules: &applicability.TemplateApplicabilityRules{
			ApplicableCompanyClasses: []applicability.CompanyClass{applicability.CompanyClassListed},
			ApplicableSectors:        []applicability.BusinessSector{applicability.BusinessSectorCommercial},
			DeadlineDays:             5,
			DeadlineDayType:          "calendar",
		},
	}
	norm := disclosureapp.NormalizeTemplateImportV1(tpl)
	errs, _, _, _ := disclosureapp.ValidateImportTemplate(norm, time.Now(), nil, nil, nil)
	found := false
	for _, e := range errs {
		if e.Code == "INVALID_PERIODICITY" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected INVALID_PERIODICITY, got %+v", errs)
	}
}

func TestIrregularWithoutPeriodicityPassesDeadlineRule(t *testing.T) {
	tpl := disclosureapp.TemplateImportDefinitionV1{
		Name:             "irregular ok",
		TemplateCategory: "irregular",
		DeadlineRule:     "Trong 24 giờ",
	}
	norm := disclosureapp.NormalizeTemplateImportV1(tpl)
	errs, _, _, _ := disclosureapp.ValidateImportTemplate(norm, time.Now(), nil, nil, nil)
	for _, e := range errs {
		if e.Code == "DEADLINE_RULE_REQUIRED" || e.Code == "INVALID_PERIODICITY" {
			t.Fatalf("unexpected %s", e.Code)
		}
	}
}

func TestIrregularDailyPeriodicityRejected(t *testing.T) {
	tpl := disclosureapp.TemplateImportDefinitionV1{
		Name:             "irregular daily",
		TemplateCategory: "irregular",
		Periodicity:      "daily",
		DeadlineRule:     "Trong 24 giờ",
	}
	errs, _, _, _ := disclosureapp.ValidateImportTemplate(&tpl, time.Now(), nil, nil, nil)
	found := false
	for _, e := range errs {
		if e.Code == "INVALID_PERIODICITY" {
			found = true
		}
	}
	if !found {
		t.Fatalf("errs=%+v", errs)
	}
}

func TestSchemaParityConditionalPeriodicity(t *testing.T) {
	iam, err := os.ReadFile("../../../docs/schema/template-import-v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	body := string(iam)
	if !strings.Contains(body, `"allOf"`) {
		t.Fatal("iam schema missing allOf")
	}
	if strings.Contains(body, `yearly", "event_based", "ad_hoc"`) {
		t.Fatal("iam schema still has a shared seven-value enum")
	}
	fe, err := os.ReadFile("../../../../cobo_web_design/docs/schema/template-import-v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(fe) != string(iam) && !strings.Contains(string(fe), `"allOf"`) {
		t.Fatal("frontend schema missing allOf")
	}
	if strings.Contains(string(fe), `"deadline_rule"`) && strings.Contains(string(fe), `"required": ["name", "template_category", "deadline_rule"]`) {
		t.Fatal("frontend schema still requires deadline_rule unconditionally")
	}
}
