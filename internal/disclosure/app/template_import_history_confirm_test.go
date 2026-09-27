package app_test

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestImportHistoryFlagParser(t *testing.T) {
	key := "CMS_TEMPLATE_IMPORT_HISTORY_ENABLED"
	cases := []struct {
		name string
		set  bool
		val  string
		on   bool
	}{
		{name: "unset", on: false},
		{name: "false", set: true, val: "false", on: false},
		{name: "FALSE", set: true, val: "FALSE", on: false},
		{name: "zero", set: true, val: "0", on: false},
		{name: "no", set: true, val: "no", on: false},
		{name: "NO", set: true, val: "NO", on: false},
		{name: "true", set: true, val: "true", on: true},
		{name: "TRUE", set: true, val: "TRUE", on: true},
		{name: "one", set: true, val: "1", on: true},
		{name: "yes", set: true, val: "yes", on: true},
		{name: "YES", set: true, val: "YES", on: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.set {
				t.Setenv(key, tc.val)
			} else {
				t.Setenv(key, "sentinel")
				_ = os.Unsetenv(key)
			}
			if disclosureapp.ImportHistoryEnabled() != tc.on {
				t.Fatalf("value %q enabled=%v want %v", tc.val, disclosureapp.ImportHistoryEnabled(), tc.on)
			}
		})
	}
}

func TestFlagOffConfirmDoesNotWriteHistory(t *testing.T) {
	t.Setenv("CMS_TEMPLATE_IMPORT_HISTORY_ENABLED", "false")
	svc, repo, sub := setupConfirmTestService()
	norm := samplePeriodicNormalizedTemplate()
	tok := issueValidTokenForDefinition(t, norm, sub.UserID)
	resp, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
		Subject:            sub,
		ValidationToken:    tok,
		TargetTypeID:       "flag-off-target",
		TargetName:         norm.Name,
		NormalizedTemplate: *norm,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.IsActive || resp.IsReleased || resp.VersionNo != 1 {
		t.Fatalf("draft lifecycle changed: %+v", resp)
	}
	row, err := repo.GetImportAttemptByID(context.Background(), "missing")
	if err != nil || row != nil {
		t.Fatalf("flag off must not create an attempt row, row=%v err=%v", row, err)
	}
	_, err = svc.ListTemplateImportHistory(context.Background(), disclosureapp.ListTemplateImportHistoryRequest{Subject: sub})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeFeatureDisabled || he.HTTPStatus != 404 {
		t.Fatalf("history read: %+v", err)
	}
}

func TestFlagOnConfirmSuccessAndReplay(t *testing.T) {
	t.Setenv("CMS_TEMPLATE_IMPORT_HISTORY_ENABLED", "true")
	svc, repo, sub := setupConfirmTestService()
	validated := validateSample(t, svc, sub, *samplePeriodicNormalizedTemplate(), "exact.json")
	if validated.ImportAttemptID == "" || validated.ValidationToken == "" || validated.Preview == nil || validated.Preview.NormalizedTemplate == nil {
		t.Fatalf("validate response incomplete: %+v", validated)
	}
	norm := validated.Preview.NormalizedTemplate
	resp, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
		Subject:            sub,
		ImportAttemptID:    validated.ImportAttemptID,
		ValidationToken:    validated.ValidationToken,
		TargetTypeID:       "flag-on-success",
		TargetName:         norm.Name,
		NormalizedTemplate: *norm,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.IsActive || resp.IsReleased || resp.VersionNo != 1 || resp.TypeID != "flag-on-success" {
		t.Fatalf("unexpected draft: %+v", resp)
	}
	detail, err := svc.GetTemplateImportHistory(context.Background(), disclosureapp.GetTemplateImportHistoryRequest{Subject: sub, ID: validated.ImportAttemptID})
	if err != nil {
		t.Fatal(err)
	}
	if detail.Status != disclosureapp.ImportAttemptStatusConfirmed || detail.CreatedTypeID != "flag-on-success" {
		t.Fatalf("history row: %+v", detail)
	}
	stored, err := repo.GetImportAttemptByID(context.Background(), validated.ImportAttemptID)
	if err != nil || stored == nil {
		t.Fatal(err)
	}
	if stored.ValidationTokenSHA256 == "" || stored.FileSHA256 == "" || stored.CanonicalPayloadSHA256 == "" {
		t.Fatal("hash columns must be set")
	}
	if stored.ValidationTokenSHA256 == validated.ValidationToken {
		t.Fatal("raw validation token must not be stored")
	}
	_, err = svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
		Subject:            sub,
		ImportAttemptID:    validated.ImportAttemptID,
		ValidationToken:    validated.ValidationToken,
		TargetTypeID:       "flag-on-success",
		TargetName:         norm.Name,
		NormalizedTemplate: *norm,
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != 409 || he.Code != perr.CodeImportAttemptAlreadyConfirmed {
		t.Fatalf("replay: %+v", err)
	}
	existsOther, _ := repo.TypeExists(context.Background(), "flag-on-second")
	if existsOther {
		t.Fatal("replay created another draft")
	}
}

func TestFlagOnMappingRequiredConfirm(t *testing.T) {
	t.Setenv("CMS_TEMPLATE_IMPORT_HISTORY_ENABLED", "true")
	svc, _, sub := setupConfirmTestService()
	def := *samplePeriodicNormalizedTemplate()
	def.Workflow.Steps[0].DepartmentID = "portable-missing"
	def.Workflow.Steps[0].DepartmentName = "Phòng chưa có"
	validated := validateSample(t, svc, sub, def, "map.json")
	if !validated.MappingRequired || validated.CanConfirm {
		t.Fatalf("expected mapping required and can_confirm false: %+v", validated)
	}
	norm := validated.Preview.NormalizedTemplate
	source := ""
	if len(validated.RequiredMappings) > 0 {
		source = validated.RequiredMappings[0].SourceID
	}
	if source == "" {
		t.Fatal("missing mapping source")
	}
	resp, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
		Subject:            sub,
		ImportAttemptID:    validated.ImportAttemptID,
		ValidationToken:    validated.ValidationToken,
		TargetTypeID:       "flag-on-mapped",
		TargetName:         norm.Name,
		DepartmentMappings: map[string]string{source: "dept-001"},
		NormalizedTemplate: *norm,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.IsActive || resp.TypeID != "flag-on-mapped" {
		t.Fatalf("mapped confirm: %+v", resp)
	}
}

func TestFlagOnConfirmMismatchCreatesNoDraft(t *testing.T) {
	t.Setenv("CMS_TEMPLATE_IMPORT_HISTORY_ENABLED", "true")
	svc, repo, sub := setupConfirmTestService()
	validated := validateSample(t, svc, sub, *samplePeriodicNormalizedTemplate(), "base.json")
	norm := *validated.Preview.NormalizedTemplate
	cases := []struct {
		name string
		req  disclosureapp.ConfirmTemplateImportRequest
		code perr.Code
	}{
		{
			name: "unknown attempt",
			req: disclosureapp.ConfirmTemplateImportRequest{
				Subject: sub, ImportAttemptID: "00000000-0000-0000-0000-000000000000",
				ValidationToken: validated.ValidationToken, TargetTypeID: "mismatch-unknown",
				TargetName: norm.Name, NormalizedTemplate: norm,
			},
			code: perr.CodeImportAttemptNotFound,
		},
		{
			name: "unknown mapping key",
			req: disclosureapp.ConfirmTemplateImportRequest{
				Subject: sub, ImportAttemptID: validated.ImportAttemptID,
				ValidationToken: validated.ValidationToken, TargetTypeID: "mismatch-stale",
				TargetName: norm.Name, NormalizedTemplate: norm,
				DepartmentMappings: map[string]string{"not-in-snapshot": "missing-dept"},
			},
			code: perr.CodeInvalidRequest,
		},
		{
			name: "other actor owns the row",
			req: disclosureapp.ConfirmTemplateImportRequest{
				Subject: sub, ImportAttemptID: validated.ImportAttemptID,
				ValidationToken: validated.ValidationToken, TargetTypeID: "mismatch-actor",
				TargetName: norm.Name, NormalizedTemplate: norm,
			},
			code: perr.CodeImportAttemptForbidden,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.req
			if tc.name == "other actor owns the row" {
				owned, err := repo.GetImportAttemptByID(context.Background(), validated.ImportAttemptID)
				if err != nil || owned == nil {
					t.Fatal(err)
				}
				owned.ActorUserID = "someone-else"
				if err := repo.CreateImportAttempt(context.Background(), owned); err != nil {
					t.Fatal(err)
				}
			}
			_, err := svc.ConfirmTemplateImport(context.Background(), req)
			he, ok := perr.AsHTTPError(err)
			if !ok || he.Code != tc.code {
				t.Fatalf("got %+v want %s", err, tc.code)
			}
			exists, _ := repo.TypeExists(context.Background(), req.TargetTypeID)
			if exists {
				t.Fatal("mismatch created a draft")
			}
		})
	}
}

func TestFlagOnConcurrentConfirmOneDraft(t *testing.T) {
	t.Setenv("CMS_TEMPLATE_IMPORT_HISTORY_ENABLED", "true")
	svc, repo, sub := setupConfirmTestService()
	validated := validateSample(t, svc, sub, *samplePeriodicNormalizedTemplate(), "race.json")
	norm := validated.Preview.NormalizedTemplate
	req := disclosureapp.ConfirmTemplateImportRequest{
		Subject:            sub,
		ImportAttemptID:    validated.ImportAttemptID,
		ValidationToken:    validated.ValidationToken,
		TargetTypeID:       "flag-on-race",
		TargetName:         norm.Name,
		NormalizedTemplate: *norm,
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, 2)
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(n int) {
			defer wg.Done()
			<-start
			_, errs[n] = svc.ConfirmTemplateImport(context.Background(), req)
		}(i)
	}
	close(start)
	wg.Wait()
	success := 0
	conflict := 0
	for _, err := range errs {
		if err == nil {
			success++
			continue
		}
		he, ok := perr.AsHTTPError(err)
		if ok && he.HTTPStatus == 409 {
			conflict++
			continue
		}
		t.Fatalf("unexpected concurrent error: %v", err)
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d errs=%v", success, conflict, errs)
	}
	row, err := repo.GetImportAttemptByID(context.Background(), validated.ImportAttemptID)
	if err != nil || row == nil || row.Status != disclosureapp.ImportAttemptStatusConfirmed || row.CreatedTypeID != "flag-on-race" {
		t.Fatalf("row=%+v err=%v", row, err)
	}
}

func TestHistoryRetentionHidesOldRowWithoutDeletingIt(t *testing.T) {
	t.Setenv("CMS_TEMPLATE_IMPORT_HISTORY_ENABLED", "true")
	svc, repo, sub := setupConfirmTestService()
	old := validateSample(t, svc, sub, *samplePeriodicNormalizedTemplate(), "old.json")
	row, err := repo.GetImportAttemptByID(context.Background(), old.ImportAttemptID)
	if err != nil || row == nil {
		t.Fatal(err)
	}
	row.CreatedAt = row.CreatedAt.AddDate(-2, 0, 0)
	if err := repo.CreateImportAttempt(context.Background(), row); err != nil {
		t.Fatal(err)
	}
	page, err := svc.ListTemplateImportHistory(context.Background(), disclosureapp.ListTemplateImportHistoryRequest{Subject: sub, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range page.Items {
		if item.ID == old.ImportAttemptID {
			t.Fatal("row older than 365 days was returned")
		}
	}
	kept, err := repo.GetImportAttemptByID(context.Background(), old.ImportAttemptID)
	if err != nil || kept == nil {
		t.Fatal("retention must not delete the row before the purge job exists")
	}
}

func validateSample(t *testing.T, svc disclosureapp.Service, sub disclosureapp.Subject, def disclosureapp.TemplateImportDefinitionV1, filename string) *disclosureapp.ValidateTemplateImportResponse {
	t.Helper()
	raw, err := json.Marshal(disclosureapp.TemplateImportEnvelopeV1{SchemaVersion: "1.0", Template: def})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := svc.ValidateTemplateImport(context.Background(), disclosureapp.ValidateTemplateImportRequest{
		Subject:   sub,
		Filename:  filename,
		FileBytes: raw,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.ParseValid || !resp.DomainValid {
		t.Fatalf("validate failed: %+v", resp.Errors)
	}
	return resp
}
