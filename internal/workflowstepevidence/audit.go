package workflowstepevidence

import (
	"context"
	"log/slog"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
)

// Canonical audit action names (dotted, matching platform audit convention / B5 pattern).
const (
	AuditActionUpload  = "workflow.step_evidence.upload"
	AuditActionDelete  = "workflow.step_evidence.delete"
	AuditActionReplace = "workflow.step_evidence.replace"

	AuditResourceType = "workflow_step_evidence_file"
)

// Auditor is the existing platform audit append surface (optional on Service).
type Auditor interface {
	AppendAuditLog(ctx context.Context, req auditapp.AppendAuditLogRequest) error
}

type evidenceAuditContext struct {
	CompanyID          string
	DisclosureRecordID string
	WorkflowInstanceID string
	StepCode           string
	ActorUserID        string
	ActorMembershipID  string
}

func (s *Service) appendEvidenceAudit(ctx context.Context, action, resourceID string, sub wff.Subject, meta evidenceAuditContext, extra map[string]any) {
	if s.audit == nil {
		return
	}
	md := map[string]any{
		"disclosure_record_id": meta.DisclosureRecordID,
		"workflow_instance_id": meta.WorkflowInstanceID,
		"step_code":            meta.StepCode,
		"evidence_file_id":     resourceID,
	}
	for k, v := range extra {
		switch k {
		case "storage_key", "storage_path", "object_key", "absolute_path",
			"authorization", "bearer", "cookie", "password", "token", "access_token", "refresh_token":
			continue
		}
		md[k] = v
	}
	err := s.audit.AppendAuditLog(ctx, auditapp.AppendAuditLogRequest{
		ActorUserID:       sub.UserID,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            action,
		ResourceType:      AuditResourceType,
		ResourceID:        resourceID,
		Decision:          "allow",
		Metadata:          md,
	})
	// AFTER_COMMIT_BEST_EFFORT: mutation already committed; log and continue.
	if err != nil {
		s.log.Error("evidence audit append failed",
			slog.String("action", action),
			slog.String("resource_id", resourceID),
			slog.String("company_id", sub.CompanyID),
			slog.String("disclosure_record_id", meta.DisclosureRecordID),
			slog.String("workflow_instance_id", meta.WorkflowInstanceID),
			slog.String("step_code", meta.StepCode),
			slog.String("err", err.Error()),
		)
	}
}
