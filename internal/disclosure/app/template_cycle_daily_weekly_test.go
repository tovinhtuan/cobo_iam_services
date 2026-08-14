package app

import (
	"context"
	"strings"
	"testing"
)

func TestValidatePortalTemplateMatrix_AcceptsDailyWeekly(t *testing.T) {
	for _, p := range []string{PeriodicityDaily, PeriodicityWeekly, "hàng ngày", "hàng tuần"} {
		req := baseUpsertRequest()
		req.Periodicity = p
		req.DeadlineRule = "T+1"
		if err := validatePortalTemplateMatrix(&req); err != nil {
			t.Fatalf("periodicity %q: %v", p, err)
		}
		if req.Periodicity != PeriodicityDaily && req.Periodicity != PeriodicityWeekly {
			t.Fatalf("normalized periodicity=%q for input %q", req.Periodicity, p)
		}
	}
}

func TestValidatePortalTemplateMatrix_RejectsInvalidPeriodicity(t *testing.T) {
	req := baseUpsertRequest()
	req.Periodicity = "biweekly"
	req.DeadlineRule = "T+1"
	err := validatePortalTemplateMatrix(&req)
	if err == nil {
		t.Fatal("expected rejection")
	}
	if !strings.Contains(err.Error(), "template validation failed") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidatePortalTemplateMatrix_PreservesMonthlyQuarterly(t *testing.T) {
	for _, p := range []string{PeriodicityMonthly, PeriodicityQuarterly, PeriodicityYearly} {
		req := baseUpsertRequest()
		req.Periodicity = p
		req.DeadlineRule = "T+20"
		if err := validatePortalTemplateMatrix(&req); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
	}
}

func TestValidateTemplateMatrix_AcceptsDailyWeekly(t *testing.T) {
	req := baseUpsertRequest()
	req.Periodicity = PeriodicityDaily
	req.DeadlineStrategy = DeadlineStrategyFixedCycleDays
	req.DeadlineRule = "T+1"
	if err := validateTemplateMatrix(&req); err != nil {
		t.Fatalf("daily: %v", err)
	}
	req.Periodicity = PeriodicityWeekly
	if err := validateTemplateMatrix(&req); err != nil {
		t.Fatalf("weekly: %v", err)
	}
}

func TestUpsertTypeVersion_AcceptsDailyAndWeekly(t *testing.T) {
	for _, p := range []string{PeriodicityDaily, PeriodicityWeekly} {
		repo := &upsertDeadlineRepo{}
		svc := newCMSUpsertDeadlineService(repo)
		req := baseUpsertRequest()
		req.Periodicity = p
		req.DeadlineRule = "T+1"
		if _, err := svc.UpsertTypeVersion(context.Background(), req); err != nil {
			t.Fatalf("%s upsert: %v", p, err)
		}
		if !repo.upsertCalled {
			t.Fatalf("%s: repo not called", p)
		}
	}
}
