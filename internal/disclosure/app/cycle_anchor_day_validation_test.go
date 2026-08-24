package app

import (
	"context"
	"net/http"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

func TestValidateCycleAnchorDay(t *testing.T) {
	for _, day := range []int{0, 1, 28, 29, 30, 31} {
		if err := ValidateCycleAnchorDay(day); err != nil {
			t.Fatalf("day=%d: unexpected error %v", day, err)
		}
	}
	for _, day := range []int{-1, 32, 100} {
		err := ValidateCycleAnchorDay(day)
		if err == nil {
			t.Fatalf("day=%d: expected error", day)
		}
		herr, ok := err.(*perr.HTTPError)
		if !ok {
			t.Fatalf("day=%d: expected *perr.HTTPError, got %T", day, err)
		}
		if herr.HTTPStatus != http.StatusBadRequest || herr.Code != perr.CodeInvalidRequest {
			t.Fatalf("day=%d: status=%d code=%s", day, herr.HTTPStatus, herr.Code)
		}
	}
}

func TestUpdateTemplateDeadlineConfig_RejectsInvalidCycleAnchorDay(t *testing.T) {
	repo := &cmsTemplateAuthzRepo{}
	svc := newCMSTemplateAuthzService(
		[]string{permissionPlatformCMSView, permissionCMSTemplateWrite},
		repo,
	)
	_, err := svc.UpdateTemplateDeadlineConfig(context.Background(), UpdateTemplateDeadlineConfigRequest{
		Subject: Subject{UserID: "user-001", MembershipID: "member-001", CompanyID: "company-001"},
		TypeID:  "dt-001",
		DeadlineConfig: TemplateDeadlineConfig{
			T0Policy:       "system_date",
			DeadlineDays:   5,
			CycleAnchorDay: 32,
		},
	})
	if err == nil {
		t.Fatal("expected reject for day=32")
	}
	if repo.updateConfigCalled {
		t.Fatal("must not persist invalid cycle_anchor_day")
	}
}

func TestUpdateTemplateDeadlineConfig_AcceptsDay31(t *testing.T) {
	repo := &cmsTemplateAuthzRepo{}
	svc := newCMSTemplateAuthzService(
		[]string{permissionPlatformCMSView, permissionCMSTemplateWrite},
		repo,
	)
	_, err := svc.UpdateTemplateDeadlineConfig(context.Background(), UpdateTemplateDeadlineConfigRequest{
		Subject: Subject{UserID: "user-001", MembershipID: "member-001", CompanyID: "company-001"},
		TypeID:  "dt-001",
		DeadlineConfig: TemplateDeadlineConfig{
			T0Policy:       "system_date",
			DeadlineDays:   5,
			CycleAnchorDay: 31,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.updateConfigCalled {
		t.Fatal("expected persist for day=31")
	}
}

func TestUpsertCompanyTypePreference_RejectsInvalidCycleAnchorDay(t *testing.T) {
	repo := &preferenceAuthzRepo{}
	svc := NewService(repo, &permissionAuthService{permissions: map[string]struct{}{
		"disclosure.auto_create.manage": {},
	}}, idgen.UUIDv7Generator{})
	_, err := svc.UpsertCompanyTypePreference(context.Background(), UpsertCompanyTypePreferenceRequest{
		Subject:           Subject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"},
		TypeID:            "type-1",
		AutoCreateEnabled: true,
		CycleAnchorDay:    32,
	})
	if err == nil {
		t.Fatal("expected reject for company day=32")
	}
	if repo.pref != nil {
		t.Fatal("must not persist invalid company cycle_anchor_day")
	}
}

func TestUpsertCompanyTypePreference_AcceptsDay31AndClearSkipsValidation(t *testing.T) {
	repo := &preferenceAuthzRepo{}
	svc := NewService(repo, &permissionAuthService{permissions: map[string]struct{}{
		"disclosure.auto_create.manage": {},
		"disclosure.view":               {},
	}}, idgen.UUIDv7Generator{})
	sub := Subject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	_, err := svc.UpsertCompanyTypePreference(context.Background(), UpsertCompanyTypePreferenceRequest{
		Subject:           sub,
		TypeID:            "type-1",
		AutoCreateEnabled: true,
		CycleAnchorDay:    31,
	})
	if err != nil {
		t.Fatalf("day=31: %v", err)
	}
	if repo.pref == nil || repo.pref.CycleAnchorDay != 31 {
		t.Fatalf("expected day=31 stored, got %+v", repo.pref)
	}
	// clear with garbage day must still succeed (day ignored)
	_, err = svc.UpsertCompanyTypePreference(context.Background(), UpsertCompanyTypePreferenceRequest{
		Subject:          sub,
		TypeID:           "type-1",
		ClearCycleAnchor: true,
		CycleAnchorDay:   32,
	})
	if err != nil {
		t.Fatalf("clear with day=32 payload: %v", err)
	}
	if !repo.pref.ClearCycleAnchor {
		t.Fatal("expected ClearCycleAnchor")
	}
}

func TestUpsertTypeVersion_RejectsInvalidCycleAnchorDay(t *testing.T) {
	repo := &upsertDeadlineRepo{}
	svc := newCMSUpsertDeadlineService(repo)
	req := baseUpsertRequest()
	req.DeadlineConfig = &TemplateDeadlineConfig{
		DeadlineMode:   "PERIODIC",
		T0Policy:       "system_date",
		CycleAnchorDay: 32,
	}
	_, err := svc.UpsertTypeVersion(context.Background(), req)
	if err == nil {
		t.Fatal("expected reject for upsert day=32")
	}
	if repo.upsertCalled {
		t.Fatal("must not upsert invalid cycle_anchor_day")
	}
}

func TestUpsertTypeVersion_AcceptsCycleAnchorDay31(t *testing.T) {
	repo := &upsertDeadlineRepo{}
	svc := newCMSUpsertDeadlineService(repo)
	req := baseUpsertRequest()
	req.DeadlineConfig = &TemplateDeadlineConfig{
		DeadlineMode:   "PERIODIC",
		T0Policy:       "system_date",
		CycleAnchorDay: 31,
	}
	_, err := svc.UpsertTypeVersion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !repo.upsertCalled {
		t.Fatal("expected upsert for day=31")
	}
}

func TestClampDayOfMonth_UnchangedByValidationHardening(t *testing.T) {
	// Guard: this task must not alter ClampDayOfMonth semantics.
	loc := asiaHoChiMinh()
	got := ClampDayOfMonth(2026, 4, 31, loc)
	if got.Format("2006-01-02") != "2026-04-30" {
		t.Fatalf("got %s", got.Format("2006-01-02"))
	}
}
