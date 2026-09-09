package app_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

const testSecret = "test-signing-secret-for-confirm-suite-32bytes"

type confirmAuthMock struct {
	permissions []string
}

func (a *confirmAuthMock) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{Permissions: a.permissions}, nil
}

func (a *confirmAuthMock) Authorize(_ context.Context, _ authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return nil, nil
}

func (a *confirmAuthMock) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return nil, nil
}

func setupConfirmTestService() (disclosureapp.Service, *inmemory.Repository, disclosureapp.Subject) {
	_ = os.Setenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET", testSecret)
	repo := inmemory.NewRepository()
	auth := &confirmAuthMock{permissions: []string{"platform.cms.view", "cms.template.write"}}
	svc := disclosureapp.NewService(repo, auth, idgen.UUIDv7Generator{})
	sub := disclosureapp.Subject{
		UserID:    "admin-user-001",
		CompanyID: "platform",
	}
	return svc, repo, sub
}

func issueValidTokenForDefinition(t *testing.T, def *disclosureapp.TemplateImportDefinitionV1, actorID string) string {
	t.Helper()
	signer := disclosureapp.NewTemplateImportSigner(testSecret, 15*time.Minute)
	hash, err := disclosureapp.ComputeCanonicalTemplatePayloadHash(def)
	if err != nil {
		t.Fatalf("ComputeCanonicalTemplatePayloadHash failed: %v", err)
	}
	now := time.Now()
	claims := disclosureapp.TemplateImportTokenClaims{
		SchemaVersion: disclosureapp.TemplateImportSchemaVersion,
		PayloadHash:   hash,
		ActorID:       actorID,
		IssuedAt:      now.Unix(),
		ExpiresAt:     now.Add(15 * time.Minute).Unix(),
	}
	tok, err := signer.IssueToken(claims)
	if err != nil {
		t.Fatalf("signer.IssueToken failed: %v", err)
	}
	return tok
}

func samplePeriodicNormalizedTemplate() *disclosureapp.TemplateImportDefinitionV1 {
	days := 15
	cycleDay := 1
	anchorWk := "monday"
	monthInQ := 3
	return disclosureapp.NormalizeTemplateImportV1(disclosureapp.TemplateImportDefinitionV1{
		TypeID:           "sample-periodic",
		Name:             "Báo cáo Tài chính Quý Mẫu",
		TemplateCategory: "periodic",
		Periodicity:      "quarterly",
		DeadlineRule:     "T+15",
		GroupID:          "group-001",
		DisplayGroupCodes: []string{"display_groups_003"},
		DeadlineConfig: &disclosureapp.TemplateImportDeadlineConfigV1{
			FrequencyUnit:        "QUARTERLY",
			CycleAnchorDay:       &cycleDay,
			CycleAnchorWeekday:   anchorWk,
			MonthInQuarter:       &monthInQ,
			ApplicableFromMode:   "NEXT_SLOT",
			ApplicableTo:         "2028-12-31",
			DurationType:         "CALENDAR_DAYS",
			DeadlineDurationType: "CALENDAR_DAYS",
			DeadlineDays:         &days,
		},
		Workflow: &disclosureapp.TemplateImportWorkflowV1{
			Steps: []disclosureapp.TemplateImportWorkflowStepV1{
				{
					Stage:           "Lập báo cáo",
					DepartmentID:    "dept-001",
					DepartmentName:  "Phòng Tài chính - Kế toán",
					AssigneeRoleIDs: []string{"creator"},
					ProcessingDays:  10,
					DisplayOrder:    1,
					Documents: []disclosureapp.TemplateImportWorkflowDocumentV1{
						{
							Name:             "Bảng cân đối kế toán",
							Required:         true,
							TemplateFileName: "bang_can_doi.xlsx",
						},
					},
				},
				{
					Stage:           "Phê duyệt",
					DepartmentID:    "dept-003",
					DepartmentName:  "Ban Giám đốc",
					AssigneeRoleIDs: []string{"approver"},
					ProcessingDays:  5,
					DisplayOrder:    2,
				},
			},
		},
	})
}

// ---------------------------------------------------------------------------
// 1. Happy-path Variants (H1 - H8)
// ---------------------------------------------------------------------------

func TestTemplateImportConfirm_HappyPathVariants(t *testing.T) {
	svc, repo, sub := setupConfirmTestService()

	// H1: Periodic template materialization
	t.Run("H1_periodic_template_materialization", func(t *testing.T) {
		norm := samplePeriodicNormalizedTemplate()
		tok := issueValidTokenForDefinition(t, norm, sub.UserID)

		resp, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "h1-periodic-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		if err != nil {
			var he *perr.HTTPError
			if errors.As(err, &he) {
				t.Fatalf("ConfirmTemplateImport H1 failed: %v, details=%#v", err, he.Details)
			}
			t.Fatalf("ConfirmTemplateImport H1 failed: %v", err)
		}

		if resp.TypeID != "h1-periodic-target" {
			t.Errorf("TypeID = %q, want 'h1-periodic-target'", resp.TypeID)
		}
		if resp.VersionNo != 1 {
			t.Errorf("VersionNo = %d, want 1", resp.VersionNo)
		}
		if resp.IsActive != false {
			t.Errorf("IsActive = %v, want false", resp.IsActive)
		}
		if resp.IsReleased != false {
			t.Errorf("IsReleased = %v, want false", resp.IsReleased)
		}
		if resp.PortalState != "not_active" {
			t.Errorf("PortalState = %q, want 'not_active'", resp.PortalState)
		}
		if resp.RootStatus != "active" {
			t.Errorf("RootStatus = %q, want 'active'", resp.RootStatus)
		}

		// Verify repo persistence
		exists, _ := repo.TypeExists(context.Background(), "h1-periodic-target")
		if !exists {
			t.Fatal("expected type_id to exist in repository after confirm")
		}
		detail, err := repo.GetTypeVersionDetail(context.Background(), "", "h1-periodic-target", 1)
		if err != nil {
			t.Fatalf("GetTypeVersionDetail failed: %v", err)
		}
		if detail.DeadlineConfig == nil || detail.DeadlineConfig.FrequencyUnit != "QUARTERLY" {
			t.Errorf("expected frequency_unit QUARTERLY, got %#v", detail.DeadlineConfig)
		}
		if detail.DeadlineConfig.ApplicableFromMode != "NEXT_SLOT" {
			t.Errorf("ApplicableFromMode = %q, want NEXT_SLOT", detail.DeadlineConfig.ApplicableFromMode)
		}
		if detail.DeadlineConfig.ApplicableTo != "2028-12-31" {
			t.Errorf("ApplicableTo = %q, want 2028-12-31", detail.DeadlineConfig.ApplicableTo)
		}
	})

	// H2: Irregular template materialization
	t.Run("H2_irregular_template_materialization", func(t *testing.T) {
		norm := disclosureapp.NormalizeTemplateImportV1(disclosureapp.TemplateImportDefinitionV1{
			TypeID:           "sample-irregular",
			Name:             "Công bố thông tin Bất thường Mẫu",
			TemplateCategory: "irregular",
			DeadlineRule:     "24h",
			Workflow: &disclosureapp.TemplateImportWorkflowV1{
				Steps: []disclosureapp.TemplateImportWorkflowStepV1{
					{
						Stage:           "Soạn thảo",
						DepartmentID:    "dept-001",
						AssigneeRoleIDs: []string{"creator"},
						ProcessingDays:  1,
					},
				},
			},
		})
		tok := issueValidTokenForDefinition(t, norm, sub.UserID)

		resp, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "h2-irregular-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		if err != nil {
			t.Fatalf("ConfirmTemplateImport H2 failed: %v", err)
		}
		if resp.TypeID != "h2-irregular-target" || resp.VersionNo != 1 {
			t.Errorf("unexpected resp: %#v", resp)
		}
	})

	// H3: Workflow multi-step with exact department matches
	t.Run("H3_workflow_multistep_exact_departments", func(t *testing.T) {
		norm := samplePeriodicNormalizedTemplate()
		tok := issueValidTokenForDefinition(t, norm, sub.UserID)

		resp, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "h3-workflow-multistep",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		if err != nil {
			t.Fatalf("ConfirmTemplateImport H3 failed: %v", err)
		}
		if resp.VersionNo != 1 {
			t.Errorf("VersionNo = %d, want 1", resp.VersionNo)
		}

		detail, _ := repo.GetTypeVersionDetail(context.Background(), "", "h3-workflow-multistep", 1)
		steps := disclosureapp.ExtractTemplateWorkflow(detail.Blocks)
		if len(steps) != 2 {
			t.Fatalf("expected 2 workflow steps persisted, got %d", len(steps))
		}
		if steps[0].DepartmentID != "dept-001" || steps[1].DepartmentID != "dept-003" {
			t.Errorf("step departments mismatch: step1=%s, step2=%s", steps[0].DepartmentID, steps[1].DepartmentID)
		}
		// Confirm fresh server UUIDs
		if steps[0].StepID == "" || steps[1].StepID == "" || steps[0].StepID == steps[1].StepID {
			t.Errorf("step IDs must be fresh distinct UUIDs: %q vs %q", steps[0].StepID, steps[1].StepID)
		}
	})

	// H4: Workflow with Preview mapping then Confirm mapping
	t.Run("H4_workflow_with_explicit_confirm_mapping", func(t *testing.T) {
		norm := disclosureapp.NormalizeTemplateImportV1(disclosureapp.TemplateImportDefinitionV1{
			TypeID:           "sample-unmatched-dept",
			Name:             "Mẫu với Phòng ban Cần Map",
			TemplateCategory: "irregular",
			DeadlineRule:     "48h",
			Workflow: &disclosureapp.TemplateImportWorkflowV1{
				Steps: []disclosureapp.TemplateImportWorkflowStepV1{
					{
						Stage:           "Xử lý nội bộ",
						DepartmentID:    "source-dept-unknown-999",
						DepartmentName:  "Phòng Quản lý Dự án Mới",
						AssigneeRoleIDs: []string{"creator"},
						ProcessingDays:  2,
					},
				},
			},
		})
		tok := issueValidTokenForDefinition(t, norm, sub.UserID)

		resp, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "h4-mapped-target",
			TargetName:         norm.Name,
			DepartmentMappings: map[string]string{"source-dept-unknown-999": "dept-001"},
			NormalizedTemplate: *norm,
		})
		if err != nil {
			t.Fatalf("ConfirmTemplateImport H4 failed: %v", err)
		}
		if resp.TypeID != "h4-mapped-target" {
			t.Errorf("TypeID = %s, want h4-mapped-target", resp.TypeID)
		}

		detail, _ := repo.GetTypeVersionDetail(context.Background(), "", "h4-mapped-target", 1)
		steps := disclosureapp.ExtractTemplateWorkflow(detail.Blocks)
		if len(steps) != 1 || steps[0].DepartmentID != "dept-001" {
			t.Errorf("expected step department mapped to dept-001, got %#v", steps)
		}
	})

	// H5: Document metadata persisted, template_file_id empty, fresh UUID
	t.Run("H5_document_metadata_and_empty_template_file_id", func(t *testing.T) {
		norm := samplePeriodicNormalizedTemplate()
		tok := issueValidTokenForDefinition(t, norm, sub.UserID)

		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "h5-docs-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		if err != nil {
			t.Fatalf("ConfirmTemplateImport H5 failed: %v", err)
		}

		detail, _ := repo.GetTypeVersionDetail(context.Background(), "", "h5-docs-target", 1)
		steps := disclosureapp.ExtractTemplateWorkflow(detail.Blocks)
		if len(steps) == 0 || len(steps[0].Documents) == 0 {
			t.Fatal("expected document requirements in step 0")
		}
		doc := steps[0].Documents[0]
		if doc.Name != "Bảng cân đối kế toán" {
			t.Errorf("Doc Name = %q, want 'Bảng cân đối kế toán'", doc.Name)
		}
		if doc.TemplateFileName != "bang_can_doi.xlsx" {
			t.Errorf("Doc TemplateFileName = %q, want 'bang_can_doi.xlsx'", doc.TemplateFileName)
		}
		if doc.TemplateFileID != "" {
			t.Errorf("Doc TemplateFileID must be strictly empty on import, got %q", doc.TemplateFileID)
		}
		if doc.DocID == "" {
			t.Error("Doc DocID must be a generated server UUID, got empty")
		}
	})

	// H6: Valid past ApplicableTo persists Draft v1 without blocking import
	t.Run("H6_valid_past_applicable_to_persists_draft", func(t *testing.T) {
		norm := samplePeriodicNormalizedTemplate()
		norm.DeadlineConfig.ApplicableTo = "2020-01-01" // Past date
		tok := issueValidTokenForDefinition(t, norm, sub.UserID)

		resp, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "h6-past-applicable-to",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		if err != nil {
			t.Fatalf("ConfirmTemplateImport H6 failed: %v", err)
		}
		if resp.VersionNo != 1 {
			t.Errorf("VersionNo = %d, want 1", resp.VersionNo)
		}

		detail, _ := repo.GetTypeVersionDetail(context.Background(), "", "h6-past-applicable-to", 1)
		if detail.DeadlineConfig.ApplicableTo != "2020-01-01" {
			t.Errorf("ApplicableTo = %q, want '2020-01-01'", detail.DeadlineConfig.ApplicableTo)
		}
	})

	// H7: WORKING_DAYS duration type preserved
	t.Run("H7_working_days_duration_type_preserved", func(t *testing.T) {
		norm := samplePeriodicNormalizedTemplate()
		norm.DeadlineConfig.DurationType = "WORKING_DAYS"
		norm.DeadlineConfig.DeadlineDurationType = "WORKING_DAYS"
		tok := issueValidTokenForDefinition(t, norm, sub.UserID)

		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "h7-working-days",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		if err != nil {
			t.Fatalf("ConfirmTemplateImport H7 failed: %v", err)
		}

		detail, _ := repo.GetTypeVersionDetail(context.Background(), "", "h7-working-days", 1)
		if detail.DeadlineConfig.DeadlineDurationType != "WORKING_DAYS" {
			t.Errorf("DeadlineDurationType = %q, want 'WORKING_DAYS'", detail.DeadlineConfig.DeadlineDurationType)
		}
	})

	// H8: SPECIFIC_SLOT applicable_from_slot preserved
	t.Run("H8_specific_slot_preserved", func(t *testing.T) {
		norm := samplePeriodicNormalizedTemplate()
		norm.DeadlineConfig.ApplicableFromMode = "SPECIFIC_SLOT"
		norm.DeadlineConfig.ApplicableFromSlot = "2026-Q1"
		tok := issueValidTokenForDefinition(t, norm, sub.UserID)

		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "h8-specific-slot",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		if err != nil {
			t.Fatalf("ConfirmTemplateImport H8 failed: %v", err)
		}

		detail, _ := repo.GetTypeVersionDetail(context.Background(), "", "h8-specific-slot", 1)
		if detail.DeadlineConfig.ApplicableFromMode != "SPECIFIC_SLOT" || detail.DeadlineConfig.ApplicableFromSlot != "2026-Q1" {
			t.Errorf("ApplicableFrom mismatch: mode=%q, slot=%q", detail.DeadlineConfig.ApplicableFromMode, detail.DeadlineConfig.ApplicableFromSlot)
		}
	})
}

// ---------------------------------------------------------------------------
// 2. Token & Payload Tamper Negative Tests (T1 - T12, TP1 - TP10)
// ---------------------------------------------------------------------------

func TestTemplateImportConfirm_TokenAndPayloadTamperTests(t *testing.T) {
	svc, repo, sub := setupConfirmTestService()
	norm := samplePeriodicNormalizedTemplate()
	validTok := issueValidTokenForDefinition(t, norm, sub.UserID)

	// TP2: Name changed after validate -> rejected by payload hash mismatch
	t.Run("TP2_name_changed_after_validate_rejected", func(t *testing.T) {
		tampered := *norm
		tampered.Name = "Tên bị sửa đổi trái phép"

		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    validTok,
			TargetTypeID:       "tamper-name-target",
			TargetName:         tampered.Name,
			NormalizedTemplate: tampered,
		})
		if err == nil {
			t.Fatal("expected error on tampered name, got nil")
		}
		exists, _ := repo.TypeExists(context.Background(), "tamper-name-target")
		if exists {
			t.Fatal("ZERO DB WRITES failed: target type exists after rejection")
		}
	})

	// TP3: Deadline rule changed after validate -> rejected
	t.Run("TP3_deadline_rule_changed_after_validate_rejected", func(t *testing.T) {
		tampered := *norm
		tampered.DeadlineRule = "T+99"

		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    validTok,
			TargetTypeID:       "tamper-deadline-target",
			TargetName:         tampered.Name,
			NormalizedTemplate: tampered,
		})
		if err == nil {
			t.Fatal("expected error on tampered deadline_rule, got nil")
		}
		exists, _ := repo.TypeExists(context.Background(), "tamper-deadline-target")
		if exists {
			t.Fatal("ZERO DB WRITES failed: target type exists after rejection")
		}
	})

	// TP4: Workflow role changed after validate -> rejected
	t.Run("TP4_workflow_role_changed_after_validate_rejected", func(t *testing.T) {
		tampered := *norm
		tamperedWorkflow := *norm.Workflow
		tamperedSteps := make([]disclosureapp.TemplateImportWorkflowStepV1, len(tamperedWorkflow.Steps))
		copy(tamperedSteps, tamperedWorkflow.Steps)
		tamperedSteps[0].AssigneeRoleIDs = []string{"super_admin_injected"}
		tamperedWorkflow.Steps = tamperedSteps
		tampered.Workflow = &tamperedWorkflow

		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    validTok,
			TargetTypeID:       "tamper-role-target",
			TargetName:         tampered.Name,
			NormalizedTemplate: tampered,
		})
		if err == nil {
			t.Fatal("expected error on tampered workflow role, got nil")
		}
	})

	// TP5: Removed document requirement after validate -> rejected
	t.Run("TP5_removed_document_after_validate_rejected", func(t *testing.T) {
		tampered := *norm
		tamperedWorkflow := *norm.Workflow
		tamperedSteps := make([]disclosureapp.TemplateImportWorkflowStepV1, len(tamperedWorkflow.Steps))
		copy(tamperedSteps, tamperedWorkflow.Steps)
		tamperedSteps[0].Documents = nil
		tamperedWorkflow.Steps = tamperedSteps
		tampered.Workflow = &tamperedWorkflow

		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    validTok,
			TargetTypeID:       "tamper-docs-target",
			TargetName:         tampered.Name,
			NormalizedTemplate: tampered,
		})
		if err == nil {
			t.Fatal("expected error on removed document, got nil")
		}
	})

	// TP6: Token issued to Actor A, confirm attempted by Actor B -> rejected
	t.Run("TP6_actor_mismatch_rejected", func(t *testing.T) {
		actorB := disclosureapp.Subject{
			UserID:    "malicious-attacker-999",
			CompanyID: "platform",
		}
		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            actorB,
			ValidationToken:    validTok,
			TargetTypeID:       "actor-mismatch-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		if err == nil {
			t.Fatal("expected error on actor mismatch, got nil")
		}
	})

	// TP7 / T7: Expired token -> rejected
	t.Run("TP7_expired_token_rejected", func(t *testing.T) {
		signer := disclosureapp.NewTemplateImportSigner(testSecret, -1*time.Hour)
		hash, _ := disclosureapp.ComputeCanonicalTemplatePayloadHash(norm)
		expiredClaims := disclosureapp.TemplateImportTokenClaims{
			SchemaVersion: disclosureapp.TemplateImportSchemaVersion,
			PayloadHash:   hash,
			ActorID:       sub.UserID,
			IssuedAt:      time.Now().Add(-2 * time.Hour).Unix(),
			ExpiresAt:     time.Now().Add(-1 * time.Hour).Unix(),
		}
		expiredTok, _ := signer.IssueToken(expiredClaims)

		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    expiredTok,
			TargetTypeID:       "expired-token-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		if err == nil {
			t.Fatal("expected error on expired token, got nil")
		}
	})

	// TP8: Wrong purpose / wrong signature -> rejected
	t.Run("TP8_wrong_signature_tampered_token", func(t *testing.T) {
		tamperedTok := validTok[:len(validTok)-5] + "XXXXX"
		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tamperedTok,
			TargetTypeID:       "tampered-sig-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		if err == nil {
			t.Fatal("expected error on tampered signature, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// 3. Signing Secret Fail-Closed (Section 11)
// ---------------------------------------------------------------------------

func TestTemplateImportConfirm_SigningSecretFailClosed(t *testing.T) {
	origSecret := os.Getenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET")
	origMedia := os.Getenv("CMS_MEDIA_UPLOAD_SIGNING_SECRET")
	defer func() {
		_ = os.Setenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET", origSecret)
		_ = os.Setenv("CMS_MEDIA_UPLOAD_SIGNING_SECRET", origMedia)
	}()

	_ = os.Unsetenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET")
	_ = os.Unsetenv("CMS_MEDIA_UPLOAD_SIGNING_SECRET")

	repo := inmemory.NewRepository()
	auth := &confirmAuthMock{permissions: []string{"platform.cms.view", "cms.template.write"}}
	svc := disclosureapp.NewService(repo, auth, idgen.UUIDv7Generator{})
	sub := disclosureapp.Subject{UserID: "admin-1", CompanyID: "platform"}
	norm := samplePeriodicNormalizedTemplate()

	// When secret is completely empty, Confirm must fail closed
	_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
		Subject:            sub,
		ValidationToken:    "some.token",
		TargetTypeID:       "fail-closed-target",
		TargetName:         norm.Name,
		NormalizedTemplate: *norm,
	})
	if err == nil {
		t.Fatal("expected error when signing secret is unconfigured, got nil")
	}

	exists, _ := repo.TypeExists(context.Background(), "fail-closed-target")
	if exists {
		t.Fatal("DB write occurred when signing secret was unconfigured!")
	}
}

// ---------------------------------------------------------------------------
// 4. Target Name Confirm Authority (Section 8)
// ---------------------------------------------------------------------------

func TestTemplateImportConfirm_TargetNameConfirmAuthority(t *testing.T) {
	svc, repo, sub := setupConfirmTestService()
	norm := samplePeriodicNormalizedTemplate()
	tok := issueValidTokenForDefinition(t, norm, sub.UserID)

	// TargetName differing from normalized_template.name must be rejected (OPTION A)
	t.Run("target_name_mismatch_rejected", func(t *testing.T) {
		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "target-name-diff-target",
			TargetName:         "Different Name Attempt",
			NormalizedTemplate: *norm,
		})
		if err == nil {
			t.Fatal("expected error when target_name differs from normalized_template.name, got nil")
		}
		exists, _ := repo.TypeExists(context.Background(), "target-name-diff-target")
		if exists {
			t.Fatal("DB write occurred despite target_name mismatch!")
		}
	})

	// TargetName matching normalized_template.name succeeds
	t.Run("target_name_match_succeeds", func(t *testing.T) {
		resp, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "target-name-match-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		if err != nil {
			t.Fatalf("unexpected error when target_name matches: %v", err)
		}
		if resp.Name != norm.Name {
			t.Errorf("resp.Name = %q, want %q", resp.Name, norm.Name)
		}
	})
}

// ---------------------------------------------------------------------------
// 5. Replay Semantics (Section 13, 14, 15)
// ---------------------------------------------------------------------------

func TestTemplateImportConfirm_ReplaySemantics(t *testing.T) {
	svc, repo, sub := setupConfirmTestService()
	norm := samplePeriodicNormalizedTemplate()
	tok := issueValidTokenForDefinition(t, norm, sub.UserID)

	// Same token + Same Target Replay -> First 201, Second 409 Conflict
	t.Run("same_token_same_target_replay_first_201_second_409", func(t *testing.T) {
		req := disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "replay-same-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		}
		resp1, err1 := svc.ConfirmTemplateImport(context.Background(), req)
		if err1 != nil {
			t.Fatalf("first confirm failed: %v", err1)
		}
		if resp1.VersionNo != 1 {
			t.Errorf("resp1.VersionNo = %d, want 1", resp1.VersionNo)
		}

		// Second confirm with same target ID -> 409 Conflict
		_, err2 := svc.ConfirmTemplateImport(context.Background(), req)
		if err2 == nil {
			t.Fatal("second confirm must fail with 409 conflict, got nil")
		}
		var he *perr.HTTPError
		if !errors.As(err2, &he) || he.HTTPStatus != http.StatusConflict {
			t.Fatalf("expected HTTP 409 Conflict, got %v", err2)
		}

		// Verify only 1 version exists
		versions, err := repo.ListTypeVersions(context.Background(), "", "replay-same-target")
		if err != nil || len(versions) != 1 {
			t.Fatalf("expected exactly 1 version in repo, got %d (err: %v)", len(versions), err)
		}
	})

	// Same token + Different Target ID -> Allowed by stateless contract
	t.Run("same_token_different_target_id_allowed_by_contract", func(t *testing.T) {
		reqA := disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "multi-target-a",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		}
		respA, errA := svc.ConfirmTemplateImport(context.Background(), reqA)
		if errA != nil {
			t.Fatalf("confirm target A failed: %v", errA)
		}
		if respA.TypeID != "multi-target-a" {
			t.Errorf("respA.TypeID = %s, want multi-target-a", respA.TypeID)
		}

		reqB := disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "multi-target-b",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		}
		respB, errB := svc.ConfirmTemplateImport(context.Background(), reqB)
		if errB != nil {
			t.Fatalf("confirm target B failed: %v", errB)
		}
		if respB.TypeID != "multi-target-b" {
			t.Errorf("respB.TypeID = %s, want multi-target-b", respB.TypeID)
		}
	})
}

// ---------------------------------------------------------------------------
// 6. Department Mapping Matrix (M1 - M10)
// ---------------------------------------------------------------------------

func TestTemplateImportConfirm_DepartmentMappingMatrix(t *testing.T) {
	svc, repo, sub := setupConfirmTestService()

	// M4: Unknown source department + missing mapping -> rejected 400
	t.Run("M4_unknown_source_missing_mapping_rejected", func(t *testing.T) {
		norm := disclosureapp.NormalizeTemplateImportV1(disclosureapp.TemplateImportDefinitionV1{
			TypeID:           "sample-m4",
			Name:             "M4 Template",
			TemplateCategory: "irregular",
			DeadlineRule:     "24h",
			Workflow: &disclosureapp.TemplateImportWorkflowV1{
				Steps: []disclosureapp.TemplateImportWorkflowStepV1{
					{
						Stage:        "Stage 1",
						DepartmentID: "unknown-dept-x",
					},
				},
			},
		})
		tok := issueValidTokenForDefinition(t, norm, sub.UserID)

		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "m4-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
			DepartmentMappings: nil, // missing
		})
		if err == nil {
			t.Fatal("expected error on missing mapping, got nil")
		}
		exists, _ := repo.TypeExists(context.Background(), "m4-target")
		if exists {
			t.Fatal("DB write occurred on missing mapping!")
		}
	})

	// M5: Mapping target nonexistent in catalog -> rejected 400
	t.Run("M5_mapping_target_nonexistent_rejected", func(t *testing.T) {
		norm := disclosureapp.NormalizeTemplateImportV1(disclosureapp.TemplateImportDefinitionV1{
			TypeID:           "sample-m5",
			Name:             "M5 Template",
			TemplateCategory: "irregular",
			DeadlineRule:     "24h",
			Workflow: &disclosureapp.TemplateImportWorkflowV1{
				Steps: []disclosureapp.TemplateImportWorkflowStepV1{
					{
						Stage:        "Stage 1",
						DepartmentID: "source-dept-y",
					},
				},
			},
		})
		tok := issueValidTokenForDefinition(t, norm, sub.UserID)

		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "m5-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
			DepartmentMappings: map[string]string{"source-dept-y": "nonexistent-target-dept-999"},
		})
		if err == nil {
			t.Fatal("expected error on nonexistent mapping target, got nil")
		}
	})

	// M7: Extra mapping key not referenced in source template -> rejected 400
	t.Run("M7_extra_mapping_key_rejected", func(t *testing.T) {
		norm := samplePeriodicNormalizedTemplate()
		tok := issueValidTokenForDefinition(t, norm, sub.UserID)

		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "m7-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
			DepartmentMappings: map[string]string{
				"unrelated-ghost-dept": "dept-001",
			},
		})
		if err == nil {
			t.Fatal("expected error on extra unreferenced mapping key, got nil")
		}
	})

	// M8: Blank mapping target -> rejected 400
	t.Run("M8_blank_mapping_target_rejected", func(t *testing.T) {
		norm := disclosureapp.NormalizeTemplateImportV1(disclosureapp.TemplateImportDefinitionV1{
			TypeID:           "sample-m8",
			Name:             "M8 Template",
			TemplateCategory: "irregular",
			DeadlineRule:     "24h",
			Workflow: &disclosureapp.TemplateImportWorkflowV1{
				Steps: []disclosureapp.TemplateImportWorkflowStepV1{
					{Stage: "Step 1", DepartmentID: "dept-source-8"},
				},
			},
		})
		tok := issueValidTokenForDefinition(t, norm, sub.UserID)

		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "m8-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
			DepartmentMappings: map[string]string{
				"dept-source-8": "   ",
			},
		})
		if err == nil {
			t.Fatal("expected error on blank mapping target, got nil")
		}
	})

	// M9: Two source departments mapped to same target -> allowed
	t.Run("M9_two_source_departments_mapped_to_same_target_allowed", func(t *testing.T) {
		norm := disclosureapp.NormalizeTemplateImportV1(disclosureapp.TemplateImportDefinitionV1{
			TypeID:           "sample-m9",
			Name:             "M9 Template",
			TemplateCategory: "irregular",
			DeadlineRule:     "24h",
			Workflow: &disclosureapp.TemplateImportWorkflowV1{
				Steps: []disclosureapp.TemplateImportWorkflowStepV1{
					{Stage: "Step 1", DepartmentID: "dept-source-a"},
					{Stage: "Step 2", DepartmentID: "dept-source-b"},
				},
			},
		})
		tok := issueValidTokenForDefinition(t, norm, sub.UserID)

		resp, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "m9-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
			DepartmentMappings: map[string]string{
				"dept-source-a": "dept-001",
				"dept-source-b": "dept-001",
			},
		})
		if err != nil {
			t.Fatalf("ConfirmTemplateImport M9 failed: %v", err)
		}
		if resp.VersionNo != 1 {
			t.Errorf("VersionNo = %d, want 1", resp.VersionNo)
		}
	})
}

// ---------------------------------------------------------------------------
// 7. Display Group Revalidation (Section 21)
// ---------------------------------------------------------------------------

func TestTemplateImportConfirm_DisplayGroupRevalidation(t *testing.T) {
	svc, repo, sub := setupConfirmTestService()
	norm := samplePeriodicNormalizedTemplate()
	norm.DisplayGroupCodes = []string{"nonexistent_group_code_999"}
	tok := issueValidTokenForDefinition(t, norm, sub.UserID)

	_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
		Subject:            sub,
		ValidationToken:    tok,
		TargetTypeID:       "display-group-reval-target",
		TargetName:         norm.Name,
		NormalizedTemplate: *norm,
	})
	if err == nil {
		t.Fatal("expected error on nonexistent display group code, got nil")
	}
	exists, _ := repo.TypeExists(context.Background(), "display-group-reval-target")
	if exists {
		t.Fatal("DB write occurred with invalid display group code!")
	}
}

// ---------------------------------------------------------------------------
// 8. Concurrency Safety Race Test (Section 24, 25)
// ---------------------------------------------------------------------------

func TestTemplateImportConfirm_ConcurrencyRace(t *testing.T) {
	svc, repo, sub := setupConfirmTestService()
	norm := samplePeriodicNormalizedTemplate()
	tok := issueValidTokenForDefinition(t, norm, sub.UserID)

	const n = 10
	var wg sync.WaitGroup
	wg.Add(n)

	successCount := 0
	conflictCount := 0
	otherErrorCount := 0
	var mu sync.Mutex

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
				Subject:            sub,
				ValidationToken:    tok,
				TargetTypeID:       "concurrent-target-race",
				TargetName:         norm.Name,
				NormalizedTemplate: *norm,
			})
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successCount++
			} else {
				var he *perr.HTTPError
				if errors.As(err, &he) && he.HTTPStatus == http.StatusConflict {
					conflictCount++
				} else {
					otherErrorCount++
					fmt.Printf("Unexpected concurrency error: %v\n", err)
				}
			}
		}()
	}
	wg.Wait()

	if successCount != 1 {
		t.Errorf("CONCURRENT_SUCCESS_COUNT = %d, want exactly 1", successCount)
	}
	if conflictCount != n-1 {
		t.Errorf("CONCURRENT_CONFLICT_COUNT = %d, want %d", conflictCount, n-1)
	}
	if otherErrorCount != 0 {
		t.Errorf("CONCURRENT_OTHER_ERROR_COUNT = %d, want 0", otherErrorCount)
	}

	// Verify only 1 root exists in DB
	versions, err := repo.ListTypeVersions(context.Background(), "", "concurrent-target-race")
	if err != nil || len(versions) != 1 {
		t.Fatalf("expected exactly 1 version persisted in repo, got %d (err: %v)", len(versions), err)
	}
}

// ---------------------------------------------------------------------------
// 9. Rollback & Zero-Partial-Write Proof (Section 15, 50, 51)
// ---------------------------------------------------------------------------

type failingSpyRepo struct {
	disclosureapp.Repository
	failUpsert bool
	errToThrow error
}

func (r *failingSpyRepo) UpsertTypeVersion(ctx context.Context, req disclosureapp.UpsertTypeVersionRequest) (*disclosureapp.UpsertTypeVersionResponse, error) {
	if r.failUpsert {
		return nil, r.errToThrow
	}
	return r.Repository.UpsertTypeVersion(ctx, req)
}

func TestTemplateImportConfirm_RollbackAndZeroPartialWrite(t *testing.T) {
	_ = os.Setenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET", testSecret)
	rawRepo := inmemory.NewRepository()
	spy := &failingSpyRepo{
		Repository: rawRepo,
		failUpsert: true,
		errToThrow: errors.New("simulated DB transaction failure during materialization"),
	}
	auth := &confirmAuthMock{permissions: []string{"platform.cms.view", "cms.template.write"}}
	svc := disclosureapp.NewService(spy, auth, idgen.UUIDv7Generator{})
	sub := disclosureapp.Subject{UserID: "admin-rollback-test", CompanyID: "platform"}

	norm := samplePeriodicNormalizedTemplate()
	tok := issueValidTokenForDefinition(t, norm, sub.UserID)

	targetID := "rollback-zero-write-target"
	req := disclosureapp.ConfirmTemplateImportRequest{
		Subject:            sub,
		ValidationToken:    tok,
		TargetTypeID:       targetID,
		TargetName:         norm.Name,
		NormalizedTemplate: *norm,
	}

	// 1. Injected transaction failure
	_, err := svc.ConfirmTemplateImport(context.Background(), req)
	if err == nil {
		t.Fatal("expected failure on injected error, got nil")
	}

	// 2. Zero-partial-write assertions: target must NOT exist
	exists, err := rawRepo.TypeExists(context.Background(), targetID)
	if err != nil {
		t.Fatalf("TypeExists failed: %v", err)
	}
	if exists {
		t.Fatalf("ZERO_PARTIAL_WRITE failed: type_id %q was created despite transaction failure", targetID)
	}
	versions, _ := rawRepo.ListTypeVersions(context.Background(), "", targetID)
	if len(versions) != 0 {
		t.Fatalf("ZERO_PARTIAL_WRITE failed: found %d versions after failed confirm", len(versions))
	}

	// 3. Retry after rollback with the exact same token succeeds (stateless token not consumed)
	spy.failUpsert = false
	resp, err := svc.ConfirmTemplateImport(context.Background(), req)
	if err != nil {
		t.Fatalf("retry after rollback failed: %v", err)
	}
	if resp.TypeID != targetID || resp.VersionNo != 1 {
		t.Errorf("unexpected retry response: %#v", resp)
	}

	// Verify target now exists exactly once
	existsNow, _ := rawRepo.TypeExists(context.Background(), targetID)
	if !existsNow {
		t.Fatal("expected target to exist after successful retry")
	}
	versionsNow, _ := rawRepo.ListTypeVersions(context.Background(), "", targetID)
	if len(versionsNow) != 1 {
		t.Fatalf("expected exactly 1 version after retry, got %d", len(versionsNow))
	}
}

// ---------------------------------------------------------------------------
// 9. P1-01: ApplicabilityRules Fidelity Roundtrip Tests
// ---------------------------------------------------------------------------

func TestTemplateImportConfirm_ApplicabilityRulesRoundTrip(t *testing.T) {
	svc, _, sub := setupConfirmTestService()

	// A1: Omitted rules -> defaults applied
	t.Run("A1_omitted_rules_defaults", func(t *testing.T) {
		norm := samplePeriodicNormalizedTemplate()
		norm.ApplicabilityRules = nil
		tok := issueValidTokenForDefinition(t, norm, sub.UserID)

		resp, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "a1-omitted-rules",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		if err != nil {
			t.Fatalf("Confirm A1 failed: %v", err)
		}
		detail, err := svc.GetTypeVersionDetail(context.Background(), disclosureapp.GetTypeVersionDetailRequest{
			Subject:   sub,
			TypeID:    resp.TypeID,
			VersionNo: resp.VersionNo,
		})
		if err != nil {
			t.Fatalf("GetTypeVersionDetail A1 failed: %v", err)
		}
		if detail.ApplicabilityRules == nil {
			t.Fatal("expected default ApplicabilityRules to be applied, got nil")
		}
		expectedDef := applicability.DefaultGlobalRules(true)
		if len(detail.ApplicabilityRules.ApplicableCompanyClasses) != len(expectedDef.ApplicableCompanyClasses) {
			t.Errorf("expected default classes count %d, got %d", len(expectedDef.ApplicableCompanyClasses), len(detail.ApplicabilityRules.ApplicableCompanyClasses))
		}
	})

	// A2: Explicit valid non-default rules -> preserved
	t.Run("A2_explicit_valid_non_default_rules_preserved", func(t *testing.T) {
		norm := samplePeriodicNormalizedTemplate()
		customRules := &applicability.TemplateApplicabilityRules{
			ApplicableCompanyClasses: []applicability.CompanyClass{applicability.CompanyClassListed},
			ApplicableSectors:        []applicability.BusinessSector{applicability.BusinessSectorManufacturing},
			DeadlineDays:            45,
			DeadlineDayType:         "working",
			UseStructureDeadline:    true,
			DeadlineByStructure: map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{
				applicability.StructureSimpleStructure:     {Days: 45},
				applicability.StructureHasSubsidiaries:     {Days: 60},
				applicability.StructureHasSubordinateUnits: {Days: 50},
			},
		}
		norm.ApplicabilityRules = customRules
		tok := issueValidTokenForDefinition(t, norm, sub.UserID)

		resp, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "a2-custom-rules",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		if err != nil {
			t.Fatalf("Confirm A2 failed: %v", err)
		}
		detail, err := svc.GetTypeVersionDetail(context.Background(), disclosureapp.GetTypeVersionDetailRequest{
			Subject:   sub,
			TypeID:    resp.TypeID,
			VersionNo: resp.VersionNo,
		})
		if err != nil {
			t.Fatalf("GetTypeVersionDetail A2 failed: %v", err)
		}
		if detail.ApplicabilityRules == nil {
			t.Fatal("expected preserved ApplicabilityRules, got nil")
		}
		if len(detail.ApplicabilityRules.ApplicableCompanyClasses) != 1 || detail.ApplicabilityRules.ApplicableCompanyClasses[0] != applicability.CompanyClassListed {
			t.Errorf("company classes mismatch: got %v", detail.ApplicabilityRules.ApplicableCompanyClasses)
		}
		if detail.ApplicabilityRules.DeadlineDays != 45 {
			t.Errorf("DeadlineDays = %d, want 45", detail.ApplicabilityRules.DeadlineDays)
		}
		if detail.ApplicabilityRules.DeadlineDayType != "working" {
			t.Errorf("DeadlineDayType = %q, want 'working'", detail.ApplicabilityRules.DeadlineDayType)
		}
		if detail.ApplicabilityRules.DeadlineByStructure[applicability.StructureHasSubsidiaries].Days != 60 {
			t.Errorf("has_subsidiaries days = %d, want 60", detail.ApplicabilityRules.DeadlineByStructure[applicability.StructureHasSubsidiaries].Days)
		}
	})

	// A3: Periodic rules -> preserved
	t.Run("A3_periodic_rules_preserved", func(t *testing.T) {
		norm := samplePeriodicNormalizedTemplate()
		periodicRules := &applicability.TemplateApplicabilityRules{
			ApplicableCompanyClasses: []applicability.CompanyClass{applicability.CompanyClassLargePublic, applicability.CompanyClassNonLargePublic},
			ApplicableSectors:        []applicability.BusinessSector{applicability.BusinessSectorManufacturing, applicability.BusinessSectorCommercial},
			DeadlineDays:            30,
			DeadlineDayType:         "calendar",
			UseStructureDeadline:    true,
			DeadlineByStructure: map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{
				applicability.StructureSimpleStructure:     {Days: 20},
				applicability.StructureHasSubsidiaries:     {Days: 30},
				applicability.StructureHasSubordinateUnits: {Days: 25},
			},
		}
		norm.ApplicabilityRules = periodicRules
		tok := issueValidTokenForDefinition(t, norm, sub.UserID)

		resp, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "a3-periodic-rules",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		if err != nil {
			t.Fatalf("Confirm A3 failed: %v", err)
		}
		detail, err := svc.GetTypeVersionDetail(context.Background(), disclosureapp.GetTypeVersionDetailRequest{
			Subject:   sub,
			TypeID:    resp.TypeID,
			VersionNo: resp.VersionNo,
		})
		if err != nil {
			t.Fatalf("GetTypeVersionDetail A3 failed: %v", err)
		}
		if detail.ApplicabilityRules.DeadlineByStructure[applicability.StructureSimpleStructure].Days != 20 {
			t.Errorf("simple days = %d, want 20", detail.ApplicabilityRules.DeadlineByStructure[applicability.StructureSimpleStructure].Days)
		}
	})

	// A4: Irregular / global case -> preserved
	t.Run("A4_irregular_rules_preserved", func(t *testing.T) {
		norm := disclosureapp.NormalizeTemplateImportV1(disclosureapp.TemplateImportDefinitionV1{
			TypeID:           "sample-irregular-rules",
			Name:             "Mẫu Bất Thường Quy Tắc Tùy Chỉnh",
			TemplateCategory: "irregular",
			DeadlineRule:     "24h",
			ApplicabilityRules: &applicability.TemplateApplicabilityRules{
				ApplicableCompanyClasses: []applicability.CompanyClass{applicability.CompanyClassListed},
				ApplicableSectors:        []applicability.BusinessSector{applicability.BusinessSectorCommercial},
				DeadlineDays:            1,
				DeadlineDayType:         "calendar",
			},
		})
		tok := issueValidTokenForDefinition(t, norm, sub.UserID)

		resp, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    tok,
			TargetTypeID:       "a4-irregular-rules",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		if err != nil {
			t.Fatalf("Confirm A4 failed: %v", err)
		}
		detail, err := svc.GetTypeVersionDetail(context.Background(), disclosureapp.GetTypeVersionDetailRequest{
			Subject:   sub,
			TypeID:    resp.TypeID,
			VersionNo: resp.VersionNo,
		})
		if err != nil {
			t.Fatalf("GetTypeVersionDetail A4 failed: %v", err)
		}
		if len(detail.ApplicabilityRules.ApplicableCompanyClasses) != 1 || detail.ApplicabilityRules.ApplicableCompanyClasses[0] != applicability.CompanyClassListed {
			t.Errorf("company classes mismatch: got %v", detail.ApplicabilityRules.ApplicableCompanyClasses)
		}
	})

	// A5: Malformed / unsupported applicability rule -> blocked by validation
	t.Run("A5_malformed_applicability_rule_blocked", func(t *testing.T) {
		malformedDef := samplePeriodicNormalizedTemplate()
		malformedDef.ApplicabilityRules = &applicability.TemplateApplicabilityRules{
			ApplicableCompanyClasses: []applicability.CompanyClass{}, // invalid: empty
			ApplicableSectors:        []applicability.BusinessSector{applicability.BusinessSectorCommercial},
			DeadlineDays:            10,
		}
		errs, _, _, _ := disclosureapp.ValidateImportTemplate(malformedDef, time.Now(), nil, nil, nil)
		if len(errs) == 0 {
			t.Fatal("expected errors for malformed applicability rules, got none")
		}
		foundErr := false
		for _, err := range errs {
			if err.Code == "INVALID_APPLICABILITY_RULES" {
				foundErr = true
				break
			}
		}
		if !foundErr {
			t.Fatalf("expected INVALID_APPLICABILITY_RULES error code, got %v", errs)
		}
	})
}

// ---------------------------------------------------------------------------
// 10. P1-02: Token HTTP Contract Parity Matrix Tests
// ---------------------------------------------------------------------------

func TestTemplateImportConfirm_TokenHTTPContract(t *testing.T) {
	svc, repo, sub := setupConfirmTestService()
	norm := samplePeriodicNormalizedTemplate()
	validTok := issueValidTokenForDefinition(t, norm, sub.UserID)

	assertHTTPError := func(t *testing.T, err error, wantStatus int, wantCode perr.Code, targetID string) {
		t.Helper()
		if err == nil {
			t.Fatalf("expected error with status %d code %s, got nil", wantStatus, wantCode)
		}
		var he *perr.HTTPError
		if !errors.As(err, &he) {
			t.Fatalf("expected *perr.HTTPError, got %T (%v)", err, err)
		}
		if he.HTTPStatus != wantStatus {
			t.Errorf("HTTPStatus = %d, want %d", he.HTTPStatus, wantStatus)
		}
		if he.Code != wantCode {
			t.Errorf("Code = %q, want %q", he.Code, wantCode)
		}
		// Assert zero DB writes
		if targetID != "" {
			exists, _ := repo.TypeExists(context.Background(), targetID)
			if exists {
				t.Fatalf("ZERO DB WRITES failed: target %q exists in repo", targetID)
			}
		}
	}

	// 1. Missing ValidationToken -> 400 INVALID_REQUEST
	t.Run("missing_token_returns_400", func(t *testing.T) {
		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    "",
			TargetTypeID:       "missing-tok-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		assertHTTPError(t, err, http.StatusBadRequest, perr.CodeInvalidRequest, "missing-tok-target")
	})

	// 2. Missing TargetTypeID -> 400 INVALID_REQUEST
	t.Run("missing_target_type_id_returns_400", func(t *testing.T) {
		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    validTok,
			TargetTypeID:       "",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		assertHTTPError(t, err, http.StatusBadRequest, perr.CodeInvalidRequest, "")
	})

	// 3. Missing TargetName -> 400 INVALID_REQUEST
	t.Run("missing_target_name_returns_400", func(t *testing.T) {
		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    validTok,
			TargetTypeID:       "missing-name-target",
			TargetName:         "",
			NormalizedTemplate: *norm,
		})
		assertHTTPError(t, err, http.StatusBadRequest, perr.CodeInvalidRequest, "missing-name-target")
	})

	// 4. TargetName Mismatch -> 400 INVALID_REQUEST
	t.Run("target_name_mismatch_returns_400", func(t *testing.T) {
		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    validTok,
			TargetTypeID:       "name-mismatch-target",
			TargetName:         "Different Title From Normalized Template",
			NormalizedTemplate: *norm,
		})
		assertHTTPError(t, err, http.StatusBadRequest, perr.CodeInvalidRequest, "name-mismatch-target")
	})

	// 5. Invalid Token Structure -> 422 INVALID_IMPORT_TOKEN
	t.Run("invalid_token_structure_returns_422", func(t *testing.T) {
		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    "not.a.valid.jwt.token.structure",
			TargetTypeID:       "bad-struct-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		assertHTTPError(t, err, http.StatusUnprocessableEntity, perr.CodeInvalidImportToken, "bad-struct-target")
	})

	// 6. Tampered Signature -> 422 INVALID_IMPORT_TOKEN
	t.Run("tampered_signature_returns_422", func(t *testing.T) {
		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    validTok + "-tampered-sig",
			TargetTypeID:       "tampered-sig-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		assertHTTPError(t, err, http.StatusUnprocessableEntity, perr.CodeInvalidImportToken, "tampered-sig-target")
	})

	// 7. Expired Token -> 422 INVALID_IMPORT_TOKEN
	t.Run("expired_token_returns_422", func(t *testing.T) {
		signer := disclosureapp.NewTemplateImportSigner(testSecret, -1*time.Hour)
		hash, _ := disclosureapp.ComputeCanonicalTemplatePayloadHash(norm)
		expiredClaims := disclosureapp.TemplateImportTokenClaims{
			SchemaVersion: disclosureapp.TemplateImportSchemaVersion,
			PayloadHash:   hash,
			ActorID:       sub.UserID,
			IssuedAt:      time.Now().Add(-2 * time.Hour).Unix(),
			ExpiresAt:     time.Now().Add(-1 * time.Hour).Unix(),
		}
		expiredTok, _ := signer.IssueToken(expiredClaims)

		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    expiredTok,
			TargetTypeID:       "expired-tok-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		assertHTTPError(t, err, http.StatusUnprocessableEntity, perr.CodeInvalidImportToken, "expired-tok-target")
	})

	// 8. Actor Mismatch -> 422 INVALID_IMPORT_TOKEN
	t.Run("actor_mismatch_returns_422", func(t *testing.T) {
		otherActor := disclosureapp.Subject{
			UserID:    "other-actor-id-different",
			CompanyID: "platform",
		}
		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            otherActor,
			ValidationToken:    validTok,
			TargetTypeID:       "actor-mismatch-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		assertHTTPError(t, err, http.StatusUnprocessableEntity, perr.CodeInvalidImportToken, "actor-mismatch-target")
	})

	// 9. Wrong Purpose -> 422 INVALID_IMPORT_TOKEN
	t.Run("wrong_purpose_returns_422", func(t *testing.T) {
		hash, _ := disclosureapp.ComputeCanonicalTemplatePayloadHash(norm)
		claims := disclosureapp.TemplateImportTokenClaims{
			SchemaVersion: disclosureapp.TemplateImportSchemaVersion,
			PayloadHash:   hash,
			ActorID:       sub.UserID,
			IssuedAt:      time.Now().Unix(),
			ExpiresAt:     time.Now().Add(15 * time.Minute).Unix(),
		}
		claimsJSON, _ := json.Marshal(claims)
		h := hmac.New(sha256.New, []byte(testSecret))
		h.Write([]byte("wrong_purpose_domain:"))
		h.Write(claimsJSON)
		forgedSig := hex.EncodeToString(h.Sum(nil))
		wrongTok := base64.RawURLEncoding.EncodeToString(claimsJSON) + "." + forgedSig

		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    wrongTok,
			TargetTypeID:       "wrong-purpose-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		assertHTTPError(t, err, http.StatusUnprocessableEntity, perr.CodeInvalidImportToken, "wrong-purpose-target")
	})

	// 10. Wrong Schema Version -> 422 INVALID_IMPORT_TOKEN
	t.Run("wrong_schema_returns_422", func(t *testing.T) {
		hash, _ := disclosureapp.ComputeCanonicalTemplatePayloadHash(norm)
		claims := disclosureapp.TemplateImportTokenClaims{
			SchemaVersion: "99.0",
			PayloadHash:   hash,
			ActorID:       sub.UserID,
			IssuedAt:      time.Now().Unix(),
			ExpiresAt:     time.Now().Add(15 * time.Minute).Unix(),
		}
		claimsJSON, _ := json.Marshal(claims)
		claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
		mac := hmac.New(sha256.New, []byte(testSecret))
		_, _ = mac.Write([]byte(disclosureapp.TemplateImportTokenPurpose + ":" + claimsB64))
		sigB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
		wrongTok := claimsB64 + "." + sigB64

		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    wrongTok,
			TargetTypeID:       "wrong-schema-target",
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		assertHTTPError(t, err, http.StatusUnprocessableEntity, perr.CodeInvalidImportToken, "wrong-schema-target")
	})

	// 11. Payload Hash Mismatch -> 422 INVALID_IMPORT_TOKEN
	t.Run("payload_hash_mismatch_returns_422", func(t *testing.T) {
		tampered := *norm
		tampered.DeadlineRule = "T+999_TAMPERED"

		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    validTok,
			TargetTypeID:       "hash-mismatch-target",
			TargetName:         tampered.Name,
			NormalizedTemplate: tampered,
		})
		assertHTTPError(t, err, http.StatusUnprocessableEntity, perr.CodeInvalidImportToken, "hash-mismatch-target")
	})

	// 12. Target Type ID Collision -> 409 STATE_CONFLICT
	t.Run("target_type_id_collision_returns_409", func(t *testing.T) {
		// First confirm succeeds
		collidingID := "colliding-type-id-target"
		_, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    validTok,
			TargetTypeID:       collidingID,
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		if err != nil {
			t.Fatalf("First confirm failed: %v", err)
		}
		// Second confirm with same target ID -> 409 STATE_CONFLICT
		_, err = svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
			Subject:            sub,
			ValidationToken:    validTok,
			TargetTypeID:       collidingID,
			TargetName:         norm.Name,
			NormalizedTemplate: *norm,
		})
		assertHTTPError(t, err, http.StatusConflict, perr.CodeStateConflict, "")
	})
}

// ---------------------------------------------------------------------------
// 11. P1-04: Signing Secret Hardening & Missing Secret Fail-Closed Tests
// ---------------------------------------------------------------------------

func TestTemplateImportSigningSecret_NoMediaFallback(t *testing.T) {
	origImportSec := os.Getenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET")
	origMediaSec := os.Getenv("CMS_MEDIA_UPLOAD_SIGNING_SECRET")
	t.Cleanup(func() {
		_ = os.Setenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET", origImportSec)
		_ = os.Setenv("CMS_MEDIA_UPLOAD_SIGNING_SECRET", origMediaSec)
	})

	// Explicitly unset import signing secret, set media upload secret
	_ = os.Unsetenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET")
	_ = os.Setenv("CMS_MEDIA_UPLOAD_SIGNING_SECRET", "media-upload-secret-not-for-template-import")

	// 1. ResolveTemplateImportSigningSecret must NOT fallback to media secret
	resolved := disclosureapp.ResolveTemplateImportSigningSecret("")
	if resolved != "" {
		t.Fatalf("expected empty signing secret when CMS_TEMPLATE_IMPORT_SIGNING_SECRET is unset, got %q", resolved)
	}

	// 2. Signer constructed without secret fails closed
	signer := disclosureapp.NewTemplateImportSigner("", 0)

	// IssueToken fails closed
	_, err := signer.IssueToken(disclosureapp.TemplateImportTokenClaims{
		SchemaVersion: "1.0",
		PayloadHash:   "abc",
		ActorID:       "actor-1",
		IssuedAt:      time.Now().Unix(),
		ExpiresAt:     time.Now().Add(15 * time.Minute).Unix(),
	})
	if err == nil {
		t.Fatal("expected IssueToken to fail closed when secret is not configured, got nil")
	}

	// VerifyToken fails closed
	_, err = signer.VerifyToken("some.dummy.token", "actor-1", time.Now())
	if err == nil {
		t.Fatal("expected VerifyToken to fail closed when secret is not configured, got nil")
	}

	// 3. ConfirmTemplateImport fails closed without secret, zero DB writes
	svc, repo, sub := setupConfirmTestService()
	norm := samplePeriodicNormalizedTemplate()
	// Unset again inside test
	_ = os.Unsetenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET")

	targetID := "fail-closed-target"
	_, err = svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
		Subject:            sub,
		ValidationToken:    "arbitrary-token",
		TargetTypeID:       targetID,
		TargetName:         norm.Name,
		NormalizedTemplate: *norm,
	})
	if err == nil {
		t.Fatal("expected ConfirmTemplateImport to fail closed, got nil")
	}
	exists, _ := repo.TypeExists(context.Background(), targetID)
	if exists {
		t.Fatalf("target type %q must not be created when signing secret is missing", targetID)
	}
}

// ---------------------------------------------------------------------------
// 12. P1-03: Full Importable Field Roundtrip Proof
// ---------------------------------------------------------------------------

func TestTemplateImportConfirm_FullImportableFieldRoundTrip(t *testing.T) {
	_ = os.Setenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET", testSecret)
	svc, _, sub := setupConfirmTestService()

	// 1. Build a rich authoring template exercising EVERY V1 importable business field
	cycleAnchorDay := 1
	anchorWeekday := "friday"
	monthInQuarter := 3
	deadlineDays := 30
	openDays := 5

	rawDef := disclosureapp.TemplateImportDefinitionV1{
		TypeID:           "bctc-quy-full-v1",
		Name:             "Báo cáo tài chính quý đầy đủ quy cách",
		Description:      "Mẫu chuẩn CBTT báo cáo tài chính quý dành cho công ty đại chúng quy mô lớn.",
		Category:         "Báo cáo tài chính",
		TemplateCategory: "periodic",
		GroupID:          "group-001",
		DeadlineStrategy: "fixed_cycle_days",
		DeadlineRule:     "Trong vòng 30 ngày kể từ ngày kết thúc quý.",
		Periodicity:      "quarterly",
		DisplayGroupCodes: []string{"display_groups_003"},
		LegalBasis:       "Khoản 1 Điều 14 Thông tư 96/2020/TT-BTC",
		Applicability:    "Công ty đại chúng quy mô lớn và tổ chức niêm yết.",
		ImplementationContent: "Lập, soát xét, phê duyệt và ký số báo cáo tài chính trước khi công bố thông tin.",
		ImplementationNotes:   "Đảm bảo số liệu khớp với hệ thống kế toán tổng hợp.",
		SpecialCases:          "Trường hợp công ty có công ty con phải nộp BCTC hợp nhất.",
		ReportContent:         "Bảng cân đối kế toán, Báo cáo kết quả kinh doanh, Báo cáo lưu chuyển tiền tệ và Thuyết minh BCTC.",
		RequiredDocs:          "Bản scan BCTC có chữ ký người đại diện theo pháp luật và kế toán trưởng.",
		ChannelsText:          "Hệ thống CBTT UBCKNN và SGDCK",
		Beneficiaries:         "Cổ đông và nhà đầu tư đại chúng",
		ReceivingAuthorities:  "UBCKNN, SGDCK",
		Format:                "PDF có chữ ký số điện tử",
		LegalRisksText:        "Xử phạt vi phạm hành chính từ 50 đến 70 triệu đồng theo Nghị định 156/2020/NĐ-CP.",
		GeneralInfo:           "Báo cáo định kỳ quý.",
		Tags:                  []string{"bctc", "quy", "tai-chinh"},
		LegalBases: []disclosureapp.LegalBasisDTO{
			{
				ID:        "lb-001",
				Title:     "Thông tư 96/2020/TT-BTC",
				Code:      "96/2020/TT-BTC",
				Authority: "Bộ Tài chính",
				IssueDate: "2020-11-16",
				Summary:   "Hướng dẫn công bố thông tin trên thị trường chứng khoán.",
				Link:      "https://vbpl.vn/tt96-2020",
			},
		},
		Checklist: []disclosureapp.ChecklistItemDTO{
			{
				ID:      "chk-001",
				Title:   "Kiểm tra tính cân đối Bảng CĐKT",
				Owner:   "Kế toán trưởng",
				DueDate: "T+20",
				Status:  "pending",
			},
		},
		DeadlineConfig: &disclosureapp.TemplateImportDeadlineConfigV1{
			FrequencyUnit:        "QUARTERLY",
			CycleAnchorDay:       &cycleAnchorDay,
			CycleAnchorWeekday:   anchorWeekday,
			MonthInQuarter:       &monthInQuarter,
			ApplicableFromMode:   "SPECIFIC_SLOT",
			ApplicableFromSlot:   "2026-Q1",
			ApplicableTo:         "2030-12-31",
			DurationType:         "CALENDAR_DAYS",
			DeadlineDurationType: "CALENDAR_DAYS",
			DeadlineDays:         &deadlineDays,
			OpenDaysBefore:       &openDays,
		},
		ApplicabilityRules: &applicability.TemplateApplicabilityRules{
			ApplicableCompanyClasses: []applicability.CompanyClass{applicability.CompanyClassLargePublic, applicability.CompanyClassListed},
			ApplicableSectors:        []applicability.BusinessSector{applicability.BusinessSectorManufacturing, applicability.BusinessSectorCommercial},
			DeadlineDays:            30,
			DeadlineDayType:         "calendar",
			UseStructureDeadline:    true,
			DeadlineByStructure: map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{
				applicability.StructureSimpleStructure:     {Days: 20},
				applicability.StructureHasSubsidiaries:     {Days: 30},
				applicability.StructureHasSubordinateUnits: {Days: 25},
			},
		},
		Workflow: &disclosureapp.TemplateImportWorkflowV1{
			Steps: []disclosureapp.TemplateImportWorkflowStepV1{
				{
					Stage:           "Lập dự thảo BCTC",
					Description:     "Phòng kế toán tổng hợp số liệu quý.",
					Instructions:    "Thu thập báo cáo từ các chi nhánh và bộ phận.",
					DepartmentID:    "dept-finance-source",
					DepartmentName:  "Phòng Tài chính Kế toán",
					AssigneeRoleIDs: []string{"creator"},
					ProcessingDays:  10,
					DueRule:         "T+10",
					ReminderConfig: &disclosureapp.TemplateImportStepReminderConfigV1{
						Enabled:     true,
						OffsetsDays: []int{3, 1},
						TemplateKey: "reminder-bctc-draft",
					},
					Documents: []disclosureapp.TemplateImportWorkflowDocumentV1{
						{
							Name:             "Mẫu BCTC 01",
							Required:         true,
							TemplateFileName: "mau_bctc_01.xlsx",
							TemplateFileID:   "source-binary-asset-pointer-must-clear",
						},
					},
				},
				{
					Stage:           "Phê duyệt và Ký số",
					Description:     "Ban Giám đốc phê duyệt BCTC chính thức.",
					Instructions:    "Kiểm tra chữ ký số của Kế toán trưởng trước khi TGĐ ký.",
					DepartmentID:    "dept-legal-source",
					DepartmentName:  "Ban Điều hành",
					AssigneeRoleIDs: []string{"approver"},
					ProcessingDays:  5,
					DueRule:         "T+15",
					ReminderConfig: &disclosureapp.TemplateImportStepReminderConfigV1{
						Enabled:     false,
					},
				},
			},
		},
	}

	// 2. Normalize
	norm := disclosureapp.NormalizeTemplateImportV1(rawDef)

	// 3. Issue Token
	tok := issueValidTokenForDefinition(t, norm, sub.UserID)

	// 4. Confirm with department mappings
	targetID := "full-roundtrip-target-001"
	confirmReq := disclosureapp.ConfirmTemplateImportRequest{
		Subject:            sub,
		ValidationToken:    tok,
		TargetTypeID:       targetID,
		TargetName:         norm.Name,
		DepartmentMappings: map[string]string{
			"dept-finance-source": "dept-003",
			"dept-legal-source":   "dept-004",
		},
		NormalizedTemplate: *norm,
	}

	confirmResp, err := svc.ConfirmTemplateImport(context.Background(), confirmReq)
	if err != nil {
		t.Fatalf("ConfirmTemplateImport failed: %v", err)
	}
	if confirmResp.TypeID != targetID || confirmResp.VersionNo != 1 {
		t.Fatalf("unexpected confirm response: %#v", confirmResp)
	}

	// 5. Canonical Reload via GetTypeVersionDetail
	detail, err := svc.GetTypeVersionDetail(context.Background(), disclosureapp.GetTypeVersionDetailRequest{
		Subject:   sub,
		TypeID:    targetID,
		VersionNo: 1,
	})
	if err != nil {
		t.Fatalf("GetTypeVersionDetail failed: %v", err)
	}

	// 6. Comprehensive Field Fidelity Assertions
	// Identity & Taxonomy
	if detail.TypeID != targetID {
		t.Errorf("TypeID = %q, want %q", detail.TypeID, targetID)
	}
	if detail.Name != norm.Name {
		t.Errorf("Name = %q, want %q", detail.Name, norm.Name)
	}
	if detail.Description != norm.Description {
		t.Errorf("Description = %q, want %q", detail.Description, norm.Description)
	}
	if detail.Category != norm.Category {
		t.Errorf("Category = %q, want %q", detail.Category, norm.Category)
	}
	if detail.TemplateCategory != norm.TemplateCategory {
		t.Errorf("TemplateCategory = %q, want %q", detail.TemplateCategory, norm.TemplateCategory)
	}
	if detail.GroupID != norm.GroupID {
		t.Errorf("GroupID = %q, want %q", detail.GroupID, norm.GroupID)
	}
	if detail.DeadlineStrategy != norm.DeadlineStrategy {
		t.Errorf("DeadlineStrategy = %q, want %q", detail.DeadlineStrategy, norm.DeadlineStrategy)
	}
	if detail.DeadlineRule != norm.DeadlineRule {
		t.Errorf("DeadlineRule = %q, want %q", detail.DeadlineRule, norm.DeadlineRule)
	}
	if detail.Periodicity != norm.Periodicity {
		t.Errorf("Periodicity = %q, want %q", detail.Periodicity, norm.Periodicity)
	}

	// Narrative & Informational Text Fields
	// Note: LegalBasis flat column gets synchronized with the legal bases projection title
	if len(norm.LegalBases) > 0 && detail.LegalBasis != norm.LegalBases[0].Title {
		t.Errorf("LegalBasis = %q, want %q", detail.LegalBasis, norm.LegalBases[0].Title)
	}
	if detail.Applicability != norm.Applicability {
		t.Errorf("Applicability = %q, want %q", detail.Applicability, norm.Applicability)
	}
	// ImplementationContent on detail is cleared by redactEnterpriseWorkflowStepsForCMSEditor;
	// verify it was preserved in the publication candidate and enterprise_workflow block description
	if detail.ImplementationContent != "" {
		t.Errorf("ImplementationContent = %q, want empty (redacted for CMS editor)", detail.ImplementationContent)
	}
	if detail.ImplementationNotes != norm.ImplementationNotes {
		t.Errorf("ImplementationNotes = %q, want %q", detail.ImplementationNotes, norm.ImplementationNotes)
	}
	if detail.SpecialCases != norm.SpecialCases {
		t.Errorf("SpecialCases = %q, want %q", detail.SpecialCases, norm.SpecialCases)
	}
	if detail.ReportContent != norm.ReportContent {
		t.Errorf("ReportContent = %q, want %q", detail.ReportContent, norm.ReportContent)
	}
	if detail.RequiredDocs != norm.RequiredDocs {
		t.Errorf("RequiredDocs = %q, want %q", detail.RequiredDocs, norm.RequiredDocs)
	}
	if detail.ChannelsText != norm.ChannelsText {
		t.Errorf("ChannelsText = %q, want %q", detail.ChannelsText, norm.ChannelsText)
	}
	if detail.Beneficiaries != norm.Beneficiaries {
		t.Errorf("Beneficiaries = %q, want %q", detail.Beneficiaries, norm.Beneficiaries)
	}
	if detail.ReceivingAuthorities != norm.ReceivingAuthorities {
		t.Errorf("ReceivingAuthorities = %q, want %q", detail.ReceivingAuthorities, norm.ReceivingAuthorities)
	}
	// Note: Format is normalized/uppercased by channelsAndFormatFromConfig
	if strings.ToUpper(detail.Format) != strings.ToUpper(norm.Format) {
		t.Errorf("Format = %q, want %q", detail.Format, norm.Format)
	}
	if detail.LegalRisksText != norm.LegalRisksText {
		t.Errorf("LegalRisksText = %q, want %q", detail.LegalRisksText, norm.LegalRisksText)
	}
	if detail.GeneralInfo != norm.GeneralInfo {
		t.Errorf("GeneralInfo = %q, want %q", detail.GeneralInfo, norm.GeneralInfo)
	}

	// Tags & Checklist & LegalBases
	if len(detail.Tags) != len(norm.Tags) {
		t.Errorf("Tags count = %d, want %d", len(detail.Tags), len(norm.Tags))
	}
	if len(detail.LegalBases) != len(norm.LegalBases) {
		t.Errorf("LegalBases count = %d, want %d", len(detail.LegalBases), len(norm.LegalBases))
	} else {
		if detail.LegalBases[0].Title != norm.LegalBases[0].Title || detail.LegalBases[0].Code != norm.LegalBases[0].Code {
			t.Errorf("LegalBasis content mismatch: got %#v", detail.LegalBases[0])
		}
	}
	if len(detail.Checklist) != len(norm.Checklist) {
		t.Errorf("Checklist count = %d, want %d", len(detail.Checklist), len(norm.Checklist))
	} else {
		if detail.Checklist[0].Title != norm.Checklist[0].Title {
			t.Errorf("Checklist title mismatch: got %q", detail.Checklist[0].Title)
		}
	}

	// DeadlineConfig
	if detail.DeadlineConfig == nil {
		t.Fatal("expected DeadlineConfig, got nil")
	}
	if detail.DeadlineConfig.FrequencyUnit != "QUARTERLY" {
		t.Errorf("FrequencyUnit = %q, want QUARTERLY", detail.DeadlineConfig.FrequencyUnit)
	}
	if detail.DeadlineConfig.CycleAnchorDay != 1 {
		t.Errorf("CycleAnchorDay mismatch: got %d, want 1", detail.DeadlineConfig.CycleAnchorDay)
	}
	if detail.DeadlineConfig.MonthInQuarter == nil || *detail.DeadlineConfig.MonthInQuarter != 3 {
		t.Errorf("MonthInQuarter mismatch")
	}
	if detail.DeadlineConfig.ApplicableFromMode != "SPECIFIC_SLOT" || detail.DeadlineConfig.ApplicableFromSlot != "2026-Q1" {
		t.Errorf("ApplicableFrom mismatch: mode=%q, slot=%q", detail.DeadlineConfig.ApplicableFromMode, detail.DeadlineConfig.ApplicableFromSlot)
	}
	if detail.DeadlineConfig.ApplicableTo != "2030-12-31" {
		t.Errorf("ApplicableTo = %q, want '2030-12-31'", detail.DeadlineConfig.ApplicableTo)
	}
	if detail.DeadlineConfig.DeadlineDays != 30 {
		t.Errorf("DeadlineDays = %d, want 30", detail.DeadlineConfig.DeadlineDays)
	}
	if detail.DeadlineConfig.OpenDaysBeforeT != 5 {
		t.Errorf("OpenDaysBeforeT = %d, want 5", detail.DeadlineConfig.OpenDaysBeforeT)
	}

	// ApplicabilityRules
	if detail.ApplicabilityRules == nil {
		t.Fatal("expected ApplicabilityRules, got nil")
	}
	if len(detail.ApplicabilityRules.ApplicableCompanyClasses) != 2 {
		t.Errorf("ApplicableCompanyClasses count = %d, want 2", len(detail.ApplicabilityRules.ApplicableCompanyClasses))
	}
	if detail.ApplicabilityRules.DeadlineDays != 30 {
		t.Errorf("ApplicabilityRules.DeadlineDays = %d, want 30", detail.ApplicabilityRules.DeadlineDays)
	}
	if detail.ApplicabilityRules.DeadlineByStructure[applicability.StructureSimpleStructure].Days != 20 {
		t.Errorf("DeadlineByStructure simple days = %d, want 20", detail.ApplicabilityRules.DeadlineByStructure[applicability.StructureSimpleStructure].Days)
	}

	// Canonical Blocks & Business Content Fidelity
	if len(detail.Blocks) != 6 {
		t.Fatalf("expected 6 canonical blocks, got %d", len(detail.Blocks))
	}
	blockByKey := make(map[string]disclosureapp.TemplateBlockDTO)
	for _, b := range detail.Blocks {
		blockByKey[b.BlockKey] = b
		if b.BlockID == "" {
			t.Errorf("BlockID must be a non-empty server-generated UUID for block %s", b.BlockKey)
		}
	}
	// Verify each canonical block has preserved imported business content
	// Note: legal_basis block description gets synchronized with legal bases projection title
	if b, ok := blockByKey["legal_basis"]; !ok || b.Description != norm.LegalBases[0].Title {
		t.Errorf("legal_basis block content mismatch: got %q, want %q", b.Description, norm.LegalBases[0].Title)
	}
	if b, ok := blockByKey["disclosure_content"]; !ok || b.Description != norm.ReportContent {
		t.Errorf("disclosure_content block content mismatch: got %q, want %q", b.Description, norm.ReportContent)
	}
	if b, ok := blockByKey["deadline"]; !ok || b.Description != norm.DeadlineRule {
		t.Errorf("deadline block content mismatch: got %q, want %q", b.Description, norm.DeadlineRule)
	}
	// channels_and_format description is synchronized with channelsText and format
	expectedChannelsDesc := norm.ChannelsText
	if norm.Format != "" {
		expectedChannelsDesc += "\nFormat: " + norm.Format
	}
	if b, ok := blockByKey["channels_and_format"]; !ok || b.Description != expectedChannelsDesc {
		t.Errorf("channels_and_format block content mismatch: got %q, want %q", b.Description, expectedChannelsDesc)
	}
	if b, ok := blockByKey["legal_risks"]; !ok || b.Description != norm.LegalRisksText {
		t.Errorf("legal_risks block content mismatch: got %q, want %q", b.Description, norm.LegalRisksText)
	}
	// enterprise_workflow description is redacted for CMS editor in GetTypeVersionDetail
	if b, ok := blockByKey["enterprise_workflow"]; !ok || b.Description != "" {
		t.Errorf("enterprise_workflow block description mismatch: got %q, want empty (redacted for CMS editor)", b.Description)
	}

	// Workflow Steps & Document Requirements:
	// Pinned workflow manifest contains the true published/persisted steps
	if detail.WorkflowManifest == nil {
		t.Fatal("expected WorkflowManifest to be populated")
	}
	manifestSteps := detail.WorkflowManifest.Steps
	if len(manifestSteps) != 2 {
		t.Fatalf("expected 2 workflow steps in manifest, got %d", len(manifestSteps))
	}
	// Step 1: department mapped to dept-003
	if manifestSteps[0].DepartmentID != "dept-003" {
		t.Errorf("step 1 DepartmentID = %q, want dept-003", manifestSteps[0].DepartmentID)
	}
	if manifestSteps[0].Stage != "Lập dự thảo BCTC" {
		t.Errorf("step 1 Stage = %q, want 'Lập dự thảo BCTC'", manifestSteps[0].Stage)
	}
	if manifestSteps[0].ProcessingDays != 10 {
		t.Errorf("step 1 ProcessingDays = %d, want 10", manifestSteps[0].ProcessingDays)
	}
	if manifestSteps[0].DueRule != "T+10" {
		t.Errorf("step 1 DueRule = %q, want 'T+10'", manifestSteps[0].DueRule)
	}
	if manifestSteps[0].ReminderConfig.Enabled != true || len(manifestSteps[0].ReminderConfig.DaysBefore) != 2 {
		t.Errorf("step 1 ReminderConfig mismatch: %#v", manifestSteps[0].ReminderConfig)
	}
	// Documents: TemplateFileName preserved, TemplateFileID cleared to ""
	if len(manifestSteps[0].Documents) != 1 {
		t.Fatalf("expected 1 document in step 1, got %d", len(manifestSteps[0].Documents))
	}
	if manifestSteps[0].Documents[0].Name != "Mẫu BCTC 01" {
		t.Errorf("document name = %q, want 'Mẫu BCTC 01'", manifestSteps[0].Documents[0].Name)
	}
	if manifestSteps[0].Documents[0].TemplateFileName != "mau_bctc_01.xlsx" {
		t.Errorf("document TemplateFileName = %q, want 'mau_bctc_01.xlsx'", manifestSteps[0].Documents[0].TemplateFileName)
	}
	if manifestSteps[0].Documents[0].TemplateFileID != "" {
		t.Errorf("document TemplateFileID must be empty string, got %q", manifestSteps[0].Documents[0].TemplateFileID)
	}

	// Step 2: department mapped to dept-004
	if manifestSteps[1].DepartmentID != "dept-004" {
		t.Errorf("step 2 DepartmentID = %q, want dept-004", manifestSteps[1].DepartmentID)
	}
}

// TestTemplateImportConfirm_DraftLifecycleFields verifies that confirming an imported template
// adheres to the source-defined Draft v1 lifecycle contract:
// - active_version_no = 0 (root inactive on Portal)
// - version_no = 1
// - is_released = false (draft state, mutable)
// - is_active = false (not active on Portal)
// - activated_at is populated with creation timestamp (per schema NOT NULL DEFAULT CURRENT_TIMESTAMP & UpsertTypeVersion)
// - NO Activate call is made; Portal state remains "not_active".
func TestTemplateImportConfirm_DraftLifecycleFields(t *testing.T) {
	svc, repo, sub := setupConfirmTestService()

	norm := samplePeriodicNormalizedTemplate()
	tok := issueValidTokenForDefinition(t, norm, sub.UserID)

	targetID := "draft-lifecycle-fields-001"
	confirmReq := disclosureapp.ConfirmTemplateImportRequest{
		Subject:            sub,
		ValidationToken:    tok,
		TargetTypeID:       targetID,
		TargetName:         norm.Name,
		NormalizedTemplate: *norm,
	}

	before := time.Now().UTC().Add(-1 * time.Second)
	confirmResp, err := svc.ConfirmTemplateImport(context.Background(), confirmReq)
	if err != nil {
		t.Fatalf("ConfirmTemplateImport failed: %v", err)
	}
	after := time.Now().UTC().Add(1 * time.Second)

	// 1. Response DTO lifecycle fields
	if confirmResp.TypeID != targetID {
		t.Errorf("TypeID = %q, want %q", confirmResp.TypeID, targetID)
	}
	if confirmResp.VersionNo != 1 {
		t.Errorf("VersionNo = %d, want 1", confirmResp.VersionNo)
	}
	if confirmResp.IsActive != false {
		t.Errorf("IsActive = %v, want false", confirmResp.IsActive)
	}
	if confirmResp.IsReleased != false {
		t.Errorf("IsReleased = %v, want false", confirmResp.IsReleased)
	}
	if confirmResp.PortalState != "not_active" {
		t.Errorf("PortalState = %q, want 'not_active'", confirmResp.PortalState)
	}
	if confirmResp.RootStatus != "active" {
		t.Errorf("RootStatus = %q, want 'active'", confirmResp.RootStatus)
	}
	if confirmResp.CreatedAt.Before(before) || confirmResp.CreatedAt.After(after) {
		t.Errorf("CreatedAt = %v, expected between %v and %v", confirmResp.CreatedAt, before, after)
	}

	// 2. Repository version listing lifecycle verification
	versions, err := repo.ListTypeVersions(context.Background(), "", targetID)
	if err != nil {
		t.Fatalf("ListTypeVersions failed: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}
	v1 := versions[0]
	if v1.VersionNo != 1 {
		t.Errorf("v1.VersionNo = %d, want 1", v1.VersionNo)
	}
	if v1.IsActive != false {
		t.Errorf("v1.IsActive = %v, want false (Portal must not be active)", v1.IsActive)
	}
	if v1.IsReleased != false {
		t.Errorf("v1.IsReleased = %v, want false (draft must not be released)", v1.IsReleased)
	}
	if v1.ActivatedAt.IsZero() {
		t.Errorf("v1.ActivatedAt is zero; expected creation timestamp populated per source semantics")
	}
	if v1.ActivatedAt.Before(before) || v1.ActivatedAt.After(after) {
		t.Errorf("v1.ActivatedAt = %v, expected creation timestamp between %v and %v", v1.ActivatedAt, before, after)
	}
}

