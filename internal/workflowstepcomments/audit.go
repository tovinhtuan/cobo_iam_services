package workflowstepcomments

import (
	"context"
	"log/slog"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
)

const (
	AuditActionCreate = "workflow.step_comment.create"
	AuditActionEdit   = "workflow.step_comment.edit"
	AuditActionDelete = "workflow.step_comment.delete"

	AuditResourceType = "workflow_step_comment"
)

// Auditor is the platform audit append surface (optional).
type Auditor interface {
	AppendAuditLog(ctx context.Context, req auditapp.AppendAuditLogRequest) error
}

type commentAuditMeta struct {
	CompanyID          string
	DisclosureRecordID string
	WorkflowInstanceID string
	StepCode           string
}

func (s *Service) appendCommentAudit(ctx context.Context, action, resourceID string, sub wff.Subject, meta commentAuditMeta, extra map[string]any) {
	if s.audit == nil {
		return
	}
	md := map[string]any{
		"disclosure_record_id": meta.DisclosureRecordID,
		"workflow_instance_id": meta.WorkflowInstanceID,
		"step_code":            meta.StepCode,
		"comment_id":           resourceID,
	}
	for k, v := range extra {
		switch k {
		case "body", "storage_key", "password", "token", "access_token", "refresh_token", "authorization", "bearer", "cookie":
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
	if err != nil {
		s.log.Error("workflow step comment audit append failed",
			slog.String("action", action),
			slog.String("resource_id", resourceID),
			slog.String("company_id", sub.CompanyID),
			slog.String("disclosure_record_id", meta.DisclosureRecordID),
			slog.String("err", err.Error()),
		)
	}
}
