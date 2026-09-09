package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

func TestCanonicalTemplateImportExample_SchemaAndValidateCompatibility(t *testing.T) {
	_ = os.Setenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET", "test-signing-secret-for-example-suite-32b")
	payload := disclosureapp.CanonicalTemplateImportExampleV1()
	if len(payload) == 0 {
		t.Fatal("embedded example payload empty")
	}

	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	var env disclosureapp.TemplateImportEnvelopeV1
	if err := dec.Decode(&env); err != nil {
		t.Fatalf("DisallowUnknownFields decode failed: %v", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		t.Fatalf("expected single JSON document, got trailing content err=%v", err)
	}
	if env.SchemaVersion != "1.0" {
		t.Fatalf("schema_version=%q want 1.0", env.SchemaVersion)
	}
	if env.Template.ApplicabilityRules == nil {
		t.Fatal("expected explicit applicability_rules in canonical example (Phase C1)")
	}
	if env.Template.DeadlineConfig == nil || env.Template.DeadlineConfig.ApplicableFromMode != "NEXT_SLOT" {
		t.Fatalf("expected NEXT_SLOT applicable_from_mode, got %#v", env.Template.DeadlineConfig)
	}
	if env.Template.DeadlineConfig.ApplicableTo != "" {
		t.Fatalf("expected open-ended applicable_to (omit), got %q", env.Template.DeadlineConfig.ApplicableTo)
	}
	if strings.Contains(string(payload), "template_file_id") {
		t.Fatal("example must not include template_file_id")
	}
	if strings.Contains(string(payload), `"company_id"`) {
		t.Fatal("example must not include company_id")
	}

	repo := inmemory.NewRepository()
	auth := &confirmAuthMock{permissions: []string{"platform.cms.view", "cms.template.write"}}
	svc := disclosureapp.NewService(repo, auth, idgen.UUIDv7Generator{})
	sub := disclosureapp.Subject{UserID: "u-example", MembershipID: "m1", CompanyID: "c1"}

	resp, err := svc.ValidateTemplateImport(context.Background(), disclosureapp.ValidateTemplateImportRequest{
		Subject:   sub,
		Filename:  disclosureapp.TemplateImportExampleFilename,
		FileBytes: payload,
	})
	if err != nil {
		t.Fatalf("ValidateTemplateImport: %v", err)
	}
	if !resp.ParseValid {
		t.Fatalf("parse_valid=false errors=%#v", resp.Errors)
	}
	if !resp.DomainValid {
		t.Fatalf("domain_valid=false errors=%#v", resp.Errors)
	}
	if resp.Preview == nil || resp.Preview.NormalizedTemplate == nil {
		t.Fatal("expected normalized_template in preview")
	}
	norm := resp.Preview.NormalizedTemplate
	if norm.ApplicabilityRules == nil {
		t.Fatal("applicability_rules lost after normalize")
	}
	if len(norm.ApplicabilityRules.ApplicableSectors) == 0 {
		t.Fatal("expected non-default applicability sectors to survive normalize")
	}
}

func TestGetTemplateImportExample_AuthorizedAndUnauthorized(t *testing.T) {
	repo := inmemory.NewRepository()
	authWrite := &confirmAuthMock{permissions: []string{"platform.cms.view", "cms.template.write"}}
	svc := disclosureapp.NewService(repo, authWrite, idgen.UUIDv7Generator{})
	sub := disclosureapp.Subject{UserID: "u-example", MembershipID: "m1", CompanyID: "c1"}

	resp, err := svc.GetTemplateImportExample(context.Background(), disclosureapp.GetTemplateImportExampleRequest{Subject: sub})
	if err != nil {
		t.Fatalf("GetTemplateImportExample: %v", err)
	}
	if resp.Filename != disclosureapp.TemplateImportExampleFilename {
		t.Fatalf("filename=%q", resp.Filename)
	}
	if resp.ContentType != disclosureapp.TemplateImportExampleContentType {
		t.Fatalf("content_type=%q", resp.ContentType)
	}
	if !bytes.Equal(resp.Payload, disclosureapp.CanonicalTemplateImportExampleV1()) {
		t.Fatal("payload mismatch vs embedded artifact")
	}

	authDenied := &confirmAuthMock{permissions: []string{"platform.cms.view"}}
	svcDenied := disclosureapp.NewService(repo, authDenied, idgen.UUIDv7Generator{})
	_, err = svcDenied.GetTemplateImportExample(context.Background(), disclosureapp.GetTemplateImportExampleRequest{Subject: sub})
	if err == nil {
		t.Fatal("expected permission denied")
	}
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("want 403, got %v", err)
	}
}
