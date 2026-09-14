package workflowfulfillment

import (
	"context"
	"log/slog"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
)

// Canonical audit action names (dotted, matching platform audit convention).
// Logically equivalent to WORKFLOW_DOCUMENT_FULFILLMENT_{UPLOAD,DELETE,REPLACE}.
const (
	AuditActionUpload  = "workflow.document_fulfillment.upload"
	AuditActionDelete  = "workflow.document_fulfillment.delete"
	AuditActionReplace = "workflow.document_fulfillment.replace"

	AuditResourceType = "workflow_step_document_fulfillment_file"
)

// Auditor is the existing platform audit append surface (optional on Service).
type Auditor interface {
	AppendAuditLog(ctx context.Context, req auditapp.AppendAuditLogRequest) error
}

type fulfillmentAuditContext struct {
	CompanyID             string
	DisclosureRecordID    string
	WorkflowInstanceID    string
	StepCode              string
	RequirementSnapshotID string
	ActorUserID           string
	ActorMembershipID     string
	SourceDocID           string
	RequirementName       string
}

func (s *Service) appendFulfillmentAudit(ctx context.Context, action, resourceID string, sub Subject, meta fulfillmentAuditContext, extra map[string]any) {
	if s.audit == nil {
		return
	}
	md := map[string]any{
		"disclosure_record_id":    meta.DisclosureRecordID,
		"workflow_instance_id":    meta.WorkflowInstanceID,
		"step_code":               meta.StepCode,
		"requirement_snapshot_id": meta.RequirementSnapshotID,
		"fulfillment_file_id":     resourceID,
	}
	if meta.SourceDocID != "" {
		md["source_doc_id"] = meta.SourceDocID
	}
	if meta.RequirementName != "" {
		md["requirement_name"] = meta.RequirementName
	}
	for k, v := range extra {
		// Hard: never accept client-controlled storage/secrets into audit.
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
	// AFTER_COMMIT_BEST_EFFORT: mutation already committed; log and continue (canonical platform policy).
	if err != nil {
		s.log.Error("fulfillment audit append failed",
			slog.String("action", action),
			slog.String("resource_id", resourceID),
			slog.String("company_id", sub.CompanyID),
			slog.String("disclosure_record_id", meta.DisclosureRecordID),
			slog.String("workflow_instance_id", meta.WorkflowInstanceID),
			slog.String("step_code", meta.StepCode),
			slog.String("requirement_snapshot_id", meta.RequirementSnapshotID),
			slog.String("err", err.Error()),
		)
	}
}
