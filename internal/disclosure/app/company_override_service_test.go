package app

import (
	"context"
	"net/http"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

func TestUpsertCompanyTypePreference_ClearWithStaleFields(t *testing.T) {
	repo := &preferenceAuthzRepo{
		cmsFreq: PeriodicityMonthly,
		pref: &CompanyTypePreference{
			CycleAnchorDay: 20, OverrideFrequency: "monthly", OverrideActive: boolPtr(true),
		},
	}
	svc := NewService(repo, &permissionAuthService{permissions: map[string]struct{}{
		"disclosure.auto_create.manage": {},
		"disclosure.view":               {},
	}}, idgen.UUIDv7Generator{})
	_, err := svc.UpsertCompanyTypePreference(context.Background(), UpsertCompanyTypePreferenceRequest{
		Subject:           Subject{CompanyID: "c1", MembershipID: "m1"},
		TypeID:            "t1",
		AutoCreateEnabled: true,
		CycleAnchorDay:    32,
		ClearCycleAnchor:  true,
	})
	if err != nil {
		t.Fatalf("clear must ignore stale fields: %v", err)
	}
	if repo.pref == nil || !repo.pref.ClearCycleAnchor {
		t.Fatal("expected ClearCycleAnchor write")
	}
}

func TestUpsertCompanyTypePreference_MonthlyOverrideBindsFrequency(t *testing.T) {
	repo := &preferenceAuthzRepo{cmsFreq: PeriodicityMonthly}
	svc := NewService(repo, &permissionAuthService{permissions: map[string]struct{}{
		"disclosure.auto_create.manage": {},
		"disclosure.view":               {},
	}}, idgen.UUIDv7Generator{})
	dto, err := svc.UpsertCompanyTypePreference(context.Background(), UpsertCompanyTypePreferenceRequest{
		Subject:           Subject{CompanyID: "c1", MembershipID: "m1"},
		TypeID:            "t1",
		AutoCreateEnabled: true,
		CycleAnchorDay:    20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.pref == nil || repo.pref.OverrideFrequency != "monthly" || repo.pref.OverrideActive == nil || !*repo.pref.OverrideActive {
		t.Fatalf("write binding: %+v", repo.pref)
	}
	if repo.pref.CycleAnchorDay != 20 || repo.pref.CycleAnchorMonth != 0 {
		t.Fatalf("monthly day-only: %+v", repo.pref)
	}
	if !dto.HasCycleAnchorOverride {
		t.Fatal("expected has override")
	}
}

func TestUpsertCompanyTypePreference_AutoCreateOnlyDoesNotMaterialize(t *testing.T) {
	active := true
	repo := &preferenceAuthzRepo{
		cmsFreq: PeriodicityMonthly,
		pref: &CompanyTypePreference{
			AutoCreateEnabled: true,
			// inherit — no active override
		},
	}
	svc := NewService(repo, &permissionAuthService{permissions: map[string]struct{}{
		"disclosure.auto_create.manage": {},
		"disclosure.view":               {},
	}}, idgen.UUIDv7Generator{})
	_, err := svc.UpsertCompanyTypePreference(context.Background(), UpsertCompanyTypePreferenceRequest{
		Subject:           Subject{CompanyID: "c1", MembershipID: "m1"},
		TypeID:            "t1",
		AutoCreateEnabled: false, // unrelated toggle only
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.pref.OverrideActive != nil || repo.pref.CycleAnchorDay != 0 {
		t.Fatalf("must not materialize inherit as override: %+v", repo.pref)
	}
	_ = active
}

func TestUpsertCompanyTypePreference_QuarterlyPartialRejected(t *testing.T) {
	repo := &preferenceAuthzRepo{cmsFreq: PeriodicityQuarterly}
	svc := NewService(repo, &permissionAuthService{permissions: map[string]struct{}{
		"disclosure.auto_create.manage": {},
	}}, idgen.UUIDv7Generator{})
	_, err := svc.UpsertCompanyTypePreference(context.Background(), UpsertCompanyTypePreferenceRequest{
		Subject:           Subject{CompanyID: "c1", MembershipID: "m1"},
		TypeID:            "t1",
		AutoCreateEnabled: true,
		CycleAnchorDay:    15,
	})
	if err == nil {
		t.Fatal("expected partial quarterly reject")
	}
	herr, ok := err.(*perr.HTTPError)
	if !ok || herr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("err=%v", err)
	}
}

func TestFrequencyChangeInvalidation_NoAutoReactivation(t *testing.T) {
	// Simulate: active monthly override → deactivate → CMS returns monthly → still inactive.
	pref := &CompanyTypePreference{
		CycleAnchorDay:    20,
		OverrideFrequency: "monthly",
		OverrideActive:    boolPtr(false), // after monthly→quarterly invalidation
	}
	auth := PreferenceToOverrideAuthority(pref, "monthly")
	if auth.Active {
		t.Fatal("FREQUENCY_RETURN_AUTO_REACTIVATION must be false")
	}
	cms := AnchorConfig{Day: 15}
	eff, src := ResolveEffectiveAnchor("monthly", cms, auth)
	if src != TSourceCMS || eff.Day != 15 {
		t.Fatalf("effective must be CMS 15: got=%+v src=%s", eff, src)
	}
}

func TestSameFrequencyCMSVersionKeepsOverride(t *testing.T) {
	pref := &CompanyTypePreference{
		CycleAnchorDay:    20,
		OverrideFrequency: "monthly",
		OverrideActive:    boolPtr(true),
	}
	auth := PreferenceToOverrideAuthority(pref, "monthly")
	cms := AnchorConfig{Day: 10} // CMS v2 changed day
	eff, src := ResolveEffectiveAnchor("monthly", cms, auth)
	if src != TSourceCompany || eff.Day != 20 {
		t.Fatalf("same-frequency must keep company 20: got=%+v src=%s", eff, src)
	}
}
