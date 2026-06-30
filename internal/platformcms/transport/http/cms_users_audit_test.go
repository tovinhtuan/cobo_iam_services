package http

import (
	"context"
	"sync"
	"testing"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
)

type recordingAuditSvc struct {
	mu   sync.Mutex
	logs []auditapp.AppendAuditLogRequest
}

func (r *recordingAuditSvc) AppendAuditLog(_ context.Context, req auditapp.AppendAuditLogRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, req)
	return nil
}

func TestAppendCMSAuditLog_RecordsUserAndMembership(t *testing.T) {
	audit := &recordingAuditSvc{}
	h := &Handler{auditSvc: audit}
	sub := iamapp.AccessTokenClaims{Sub: "u_actor", MembershipID: "m_actor", CompanyID: "c_001"}

	h.appendCMSAuditLog(context.Background(), sub, "cms.admin.users.create", "user", "u_new")
	h.appendCMSAuditLog(context.Background(), sub, "cms.admin.membership.create", "membership", "m_new")

	audit.mu.Lock()
	defer audit.mu.Unlock()
	if len(audit.logs) != 2 {
		t.Fatalf("expected 2 audit rows, got %d", len(audit.logs))
	}
	if audit.logs[0].Action != "cms.admin.users.create" || audit.logs[0].ResourceID != "u_new" {
		t.Fatalf("unexpected user audit: %+v", audit.logs[0])
	}
	if audit.logs[1].Action != "cms.admin.membership.create" || audit.logs[1].ResourceID != "m_new" {
		t.Fatalf("unexpected membership audit: %+v", audit.logs[1])
	}
	for _, log := range audit.logs {
		if log.ActorUserID != "u_actor" || log.Decision != "allow" {
			t.Fatalf("unexpected metadata: %+v", log)
		}
	}
}

func TestAppendCMSAuditLog_NilServiceNoOp(t *testing.T) {
	h := &Handler{auditSvc: nil}
	sub := iamapp.AccessTokenClaims{Sub: "u1"}
	// must not panic
	h.appendCMSAuditLog(context.Background(), sub, "cms.admin.users.create", "user", "u1")
}
