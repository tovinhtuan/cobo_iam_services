package app

import (
	"context"
	"net/http"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// ROLE-08 / RP-06: disclosure_type.manage is a tenant-grantable permission (a company admin can
// hand it out), so it must not open the global platform CMS template routes.
func TestCMSTemplateGates_TenantManageAliasRejected(t *testing.T) {
	svc := newCMSPermTestService([]string{permissionPlatformCMSView, permissionLegacyTemplateManage})
	ctx := context.Background()
	calls := map[string]func() error{
		"write (delete global workflow)": func() error {
			return svc.CmsDeleteGlobalWorkflow(ctx, CmsDeleteGlobalWorkflowRequest{Subject: subject(), TypeID: "type-001"})
		},
		"read (get global workflow)": func() error {
			_, err := svc.CmsGetGlobalWorkflow(ctx, CmsGetGlobalWorkflowRequest{Subject: subject(), TypeID: "type-001"})
			return err
		},
		"config write (deadline config)": func() error {
			_, err := svc.UpdateTemplateDeadlineConfig(ctx, UpdateTemplateDeadlineConfigRequest{Subject: subject(), TypeID: "type-001"})
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			err, passedGate := gateResult(call)
			he, ok := perr.AsHTTPError(err)
			if passedGate || !ok || he.HTTPStatus != http.StatusForbidden {
				t.Fatalf("platform.cms.view + disclosure_type.manage must be 403 at the gate, got err=%v passed=%v", err, passedGate)
			}
		})
	}
}

// gateResult runs call; the stub repository panics on any call, so a panic means the permission
// gate let the request through.
func gateResult(call func() error) (err error, passedGate bool) {
	defer func() {
		if recover() != nil {
			passedGate = true
		}
	}()
	err = call()
	if he, ok := perr.AsHTTPError(err); !ok || he.HTTPStatus != http.StatusForbidden {
		passedGate = true
	}
	return err, passedGate
}

func TestCMSTemplateGates_PlatformPermissionStillPasses(t *testing.T) {
	svc := newCMSPermTestService([]string{permissionPlatformCMSView, permissionCMSTemplateWrite})
	err, passed := gateResult(func() error {
		return svc.CmsDeleteGlobalWorkflow(context.Background(), CmsDeleteGlobalWorkflowRequest{Subject: subject(), TypeID: "type-001"})
	})
	if !passed {
		t.Fatalf("cms.template.write must pass the write gate, got %v", err)
	}
}
