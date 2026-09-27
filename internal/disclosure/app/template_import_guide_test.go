package app_test

import (
	"context"
	"strings"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

func TestCanonicalTemplateImportGuide_Rules(t *testing.T) {
	body := string(disclosureapp.CanonicalTemplateImportGuideV1())
	if strings.TrimSpace(body) == "" {
		t.Fatal("guide artifact is empty")
	}
	for _, needle := range []string{
		`schema_version`,
		`"1.0"`,
		"daily",
		"weekly",
		"monthly",
		"quarterly",
		"yearly",
		"deadline_rule",
		"deadline_days",
		"listed",
		"large_public",
		"non_large_public",
		"INVALID_JSON_PAYLOAD",
		"activation_ready",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("guide missing %q", needle)
		}
	}
	if strings.Contains(body, "public_listed") {
		t.Fatal("guide must not teach public_listed as a company class")
	}
	if strings.Contains(body, "Bearer ") || strings.Contains(body, "validation_token") {
		t.Fatal("guide must not contain token material")
	}
}

func TestGetTemplateImportGuide_AuthAndNoWrite(t *testing.T) {
	repo := inmemory.NewRepository()
	auth := &importAuthMock{permissions: []string{"platform.cms.view", "cms.template.write"}}
	svc := disclosureapp.NewService(repo, auth, idgen.UUIDv7Generator{})
	resp, err := svc.GetTemplateImportGuide(context.Background(), disclosureapp.GetTemplateImportGuideRequest{
		Subject: disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Filename != disclosureapp.TemplateImportGuideFilename {
		t.Fatalf("filename=%q", resp.Filename)
	}
	if resp.ContentType != disclosureapp.TemplateImportGuideContentType {
		t.Fatalf("content type=%q", resp.ContentType)
	}
	if len(resp.Payload) == 0 {
		t.Fatal("empty payload")
	}

	denied := &importAuthMock{permissions: []string{"platform.cms.view"}}
	svcDenied := disclosureapp.NewService(repo, denied, idgen.UUIDv7Generator{})
	_, err = svcDenied.GetTemplateImportGuide(context.Background(), disclosureapp.GetTemplateImportGuideRequest{
		Subject: disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
	})
	if err == nil {
		t.Fatal("expected permission error")
	}
}
