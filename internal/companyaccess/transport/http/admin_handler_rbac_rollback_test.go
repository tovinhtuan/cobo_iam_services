package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
)

func seedDirectGrantSnapshot(t *testing.T, repo *cainmem.AdminRepository, id, membership, code string) {
	t.Helper()
	raw, err := json.Marshal(configversion.RBACMatrixSnapshot{
		SchemaVersion:     configversion.RBACMatrixSnapshotSchema,
		DirectPermissions: []configversion.DirectPermissionEntry{{MembershipID: membership, PermissionCode: code}},
	})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if _, err := repo.InsertRBACMatrixSnapshot(context.Background(), caapp.InsertRBACMatrixSnapshotInput{
		ID: id, CompanyID: scopeHandlerCompany, SnapshotJSON: raw, CreatedBy: "m-1", Source: configversion.SourceMutationAPI,
	}); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
}

// ROLE-01: the rollback endpoint answers 200 with {rolled_back_from,new_version} when it applies
// the change, and with the same flat 202 body as every other approval-routed endpoint
// ({approval_id,status}) when a critical permission is involved.
func TestRollbackRBACMatrix_ResponseShapes(t *testing.T) {
	mux, repo := newScopeHandlerMux(t, scopeTenantPerms)
	seedDirectGrantSnapshot(t, repo, "v-critical", "m-1", "admin.membership.invite")      // version 1: critical
	seedDirectGrantSnapshot(t, repo, "v-plain", "m-1", "template.workflow.override.read") // version 2: not critical

	w := scopeCall(mux, "POST", "/api/v1/admin/rbac/matrix/versions/1/rollback", `{"reason":"test"}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("critical rollback: expected 202, got %d %s", w.Code, w.Body.String())
	}
	var routed struct {
		ApprovalID string `json:"approval_id"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &routed); err != nil || routed.ApprovalID == "" || routed.Status != "pending" {
		t.Fatalf("critical rollback: expected a flat {approval_id,status:pending} body, got %s", w.Body.String())
	}

	w = scopeCall(mux, "POST", "/api/v1/admin/rbac/matrix/versions/2/rollback", `{"reason":"test"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("plain rollback: expected 200, got %d %s", w.Code, w.Body.String())
	}
	var applied struct {
		RolledBackFrom int `json:"rolled_back_from"`
		NewVersion     struct {
			VersionNo int    `json:"version_no"`
			Source    string `json:"source"`
		} `json:"new_version"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &applied); err != nil || applied.RolledBackFrom != 2 || applied.NewVersion.VersionNo == 0 || applied.NewVersion.Source != configversion.SourceRollback {
		t.Fatalf("plain rollback: unexpected body %s", w.Body.String())
	}
}
