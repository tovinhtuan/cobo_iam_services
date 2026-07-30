package periodic_oneshot_test

import (
	"context"
	"strings"
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	oneshot "github.com/cobo/cobo_iam_services/internal/disclosure/app/periodic_oneshot"
)

func baseType() oneshot.TypeSnapshot {
	return oneshot.TypeSnapshot{
		TypeID:          oneshot.AllowedTypeID,
		TypeName:        "QA Monthly",
		Status:          "active",
		ActiveVersionNo: 1,
		DeadlineMode:    "PERIODIC",
		DeadlineDays:    23,
		FrequencyUnit:   "monthly",
		IsGlobal:        true,
		HasWorkflow:     true,
		ApplicabilityRules: &applicability.TemplateApplicabilityRules{
			ApplicableCompanyClasses: []applicability.CompanyClass{
				applicability.CompanyClassListed,
				applicability.CompanyClassLargePublic,
				applicability.CompanyClassNonLargePublic,
			},
			ApplicableSectors: []applicability.BusinessSector{
				applicability.BusinessSectorCommercial,
				applicability.BusinessSectorService,
				applicability.BusinessSectorManufacturing,
			},
			DeadlineDays:    23,
			DeadlineDayType: "working",
		},
	}
}

func baseProfile() applicability.CompanyApplicabilityProfile {
	return applicability.CompanyApplicabilityProfile{
		IsListed:       true,
		BusinessSector: ptrSector(applicability.BusinessSectorCommercial),
	}
}

func ptrSector(s applicability.BusinessSector) *applicability.BusinessSector { return &s }

func TestAllowlistRefuseWrongScope(t *testing.T) {
	err := oneshot.ValidateAllowlist(oneshot.Scope{TypeID: "x", CompanyID: "c_001", Period: "2026-07"})
	if err == nil || !strings.Contains(err.Error(), "MATERIALIZATION_SCOPE_NOT_ALLOWED") {
		t.Fatalf("want scope refuse, got %v", err)
	}
}

func TestEnvRefuseNonDEV(t *testing.T) {
	err := (oneshot.EnvGuard{Environment: "PROD", Database: "cobo_iam", HostAlias: "127.0.0.1", Port: "3306"}).Validate()
	if err == nil {
		t.Fatal("expected refuse")
	}
}

func TestPreviewCreatePlanAndCalculator(t *testing.T) {
	dom := oneshot.NewMemoryDomain(baseType(), baseProfile())
	eng := &oneshot.Engine{Env: oneshot.EnvGuard{Environment: "DEV", Database: "cobo_iam", HostAlias: "127.0.0.1", Port: "3306"}, Domain: dom}
	rep, err := eng.Preview(context.Background(), oneshot.Scope{
		TypeID: oneshot.AllowedTypeID, CompanyID: oneshot.AllowedCompanyID, Period: oneshot.AllowedPeriod,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Mutations != 0 {
		t.Fatalf("preview mutations=%d", rep.Mutations)
	}
	if rep.Resolved["due_date"] != "2026-07-31" {
		t.Fatalf("due=%v", rep.Resolved["due_date"])
	}
	if len(rep.PlannedActions) != 2 {
		t.Fatalf("planned=%v", rep.PlannedActions)
	}
	if rep.ConfirmToken == "" {
		t.Fatal("missing confirm token")
	}
}

func TestApplyAndIdempotentRerun(t *testing.T) {
	dom := oneshot.NewMemoryDomain(baseType(), baseProfile())
	eng := &oneshot.Engine{Env: oneshot.EnvGuard{Environment: "DEV", Database: "cobo_iam", HostAlias: "127.0.0.1", Port: "3306"}, Domain: dom}
	scope := oneshot.Scope{TypeID: oneshot.AllowedTypeID, CompanyID: oneshot.AllowedCompanyID, Period: oneshot.AllowedPeriod}
	prev, err := eng.Preview(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	apply1, err := eng.Apply(context.Background(), scope, prev.ConfirmToken)
	if err != nil {
		t.Fatal(err)
	}
	if apply1.Status != oneshot.StatusMaterialized || !apply1.RecordCreated || apply1.Mutations < 1 {
		t.Fatalf("apply1=%+v", apply1)
	}
	prev2, err := eng.Preview(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	apply2, err := eng.Apply(context.Background(), scope, prev2.ConfirmToken)
	if err != nil {
		t.Fatal(err)
	}
	if apply2.Status != oneshot.StatusNoOp || apply2.Mutations != 0 || apply2.RecordCreated {
		t.Fatalf("apply2=%+v", apply2)
	}
	if len(dom.Cycles) != 1 {
		t.Fatalf("cycles=%d", len(dom.Cycles))
	}
}

func TestApplyRollbackOnCreateFailure(t *testing.T) {
	dom := oneshot.NewMemoryDomain(baseType(), baseProfile())
	dom.FailCreate = true
	eng := &oneshot.Engine{Env: oneshot.EnvGuard{Environment: "DEV", Database: "cobo_iam", HostAlias: "127.0.0.1", Port: "3306"}, Domain: dom}
	scope := oneshot.Scope{TypeID: oneshot.AllowedTypeID, CompanyID: oneshot.AllowedCompanyID, Period: oneshot.AllowedPeriod}
	prev, err := eng.Preview(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	_, err = eng.Apply(context.Background(), scope, prev.ConfirmToken)
	if err == nil {
		t.Fatal("expected failure")
	}
	if len(dom.Cycles) != 0 {
		t.Fatalf("expected rollback delete cycle, got %d", len(dom.Cycles))
	}
}

func TestTokenMismatchRefuse(t *testing.T) {
	dom := oneshot.NewMemoryDomain(baseType(), baseProfile())
	eng := &oneshot.Engine{Env: oneshot.EnvGuard{Environment: "DEV", Database: "cobo_iam", HostAlias: "127.0.0.1", Port: "3306"}, Domain: dom}
	scope := oneshot.Scope{TypeID: oneshot.AllowedTypeID, CompanyID: oneshot.AllowedCompanyID, Period: oneshot.AllowedPeriod}
	_, err := eng.Apply(context.Background(), scope, "deadbeefdeadbeefdeadbeefdeadbeef")
	if err == nil {
		t.Fatal("expected token mismatch")
	}
}

func TestDeadlineDaysDriftRefuse(t *testing.T) {
	ts := baseType()
	ts.DeadlineDays = 20
	ts.ApplicabilityRules.DeadlineDays = 20
	dom := oneshot.NewMemoryDomain(ts, baseProfile())
	eng := &oneshot.Engine{Env: oneshot.EnvGuard{Environment: "DEV", Database: "cobo_iam", HostAlias: "127.0.0.1", Port: "3306"}, Domain: dom}
	_, err := eng.Preview(context.Background(), oneshot.Scope{
		TypeID: oneshot.AllowedTypeID, CompanyID: oneshot.AllowedCompanyID, Period: oneshot.AllowedPeriod,
	})
	if err == nil || !strings.Contains(err.Error(), "REFUSE") {
		t.Fatalf("want refuse, got %v", err)
	}
}

func TestWorkingDaysCalculatorMatchesExpectedDue(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	_ = loc
	calc := disclosureapp.NewDeadlineCalculator(nil)
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, loc)
	due, err := calc.AddDurationInclusive(context.Background(), start, 23, disclosureapp.DurationTypeWorkingDays)
	if err != nil {
		t.Fatal(err)
	}
	if due.Format("2006-01-02") != "2026-07-31" {
		t.Fatalf("got %s", due.Format("2006-01-02"))
	}
}
