package app

import (
	"context"
	"testing"

	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

func TestUpsertCompanyTypePreference_ClearCycleAnchor(t *testing.T) {
	repo := &preferenceAuthzRepo{pref: &CompanyTypePreference{
		CompanyID: "co-1", TypeID: "type-1", AutoCreateEnabled: true,
		CycleAnchorMonth: 10, CycleAnchorDay: 5,
	}}
	svc := NewService(repo, &permissionAuthService{permissions: map[string]struct{}{
		"disclosure.auto_create.manage": {},
		"disclosure.view":               {},
	}}, idgen.UUIDv7Generator{})
	sub := Subject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	dto, err := svc.UpsertCompanyTypePreference(context.Background(), UpsertCompanyTypePreferenceRequest{
		Subject:           sub,
		TypeID:            "type-1",
		AutoCreateEnabled: true,
		ClearCycleAnchor:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.pref == nil || !repo.pref.ClearCycleAnchor {
		t.Fatalf("expected ClearCycleAnchor on write, pref=%+v", repo.pref)
	}
	// After clear, Get returns cleared anchors when repo stores clear result.
	repo.pref.CycleAnchorMonth = 0
	repo.pref.CycleAnchorDay = 0
	repo.pref.ClearCycleAnchor = false
	dto, err = svc.GetCompanyTypePreference(context.Background(), GetCompanyTypePreferenceRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"},
		TypeID:  "type-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = dto
	if dto.HasCycleAnchorOverride {
		t.Fatal("expected no override after clear")
	}
}
