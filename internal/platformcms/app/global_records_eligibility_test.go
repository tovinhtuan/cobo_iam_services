package app

import (
	"context"
	"testing"
	"time"
)

func TestMaterialize_EligibilitySkipsAndNotStarted(t *testing.T) {
	repo := NewMemoryRepository()
	repo.SeedTemplate(TemplateInfo{TypeID: "tpl-el", Scope: "global", ActiveVersionNo: 1, PortalState: "active"})
	repo.SetCompanies([]string{"c_ok", "c_off", "c_inactive"})
	repo.SetEligibilitySkip("c_off", ReasonAutoCreateDisabled)
	repo.SetEligibilitySkip("c_inactive", ReasonCompanyInactive)
	svc := NewGlobalRecordsService(repo, &fixedID{}, fixedClock{t: time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)}, allowAll)
	actor := Actor{UserID: "u1", MembershipID: "m1", CompanyID: "cms"}

	rec, err := svc.Create(context.Background(), actor, "tpl-el", "SMOKE", "s", "body", "quarterly:2026-Q3")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Publish(context.Background(), actor, rec.ID); err != nil {
		t.Fatal(err)
	}

	preview, err := svc.Materialize(context.Background(), actor, rec.ID, MaterializeRequest{Mode: "full", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Created != 1 {
		t.Fatalf("dry-run created want 1 got %d (skipped=%d)", preview.Created, preview.Skipped)
	}
	if preview.Skipped < 2 {
		t.Fatalf("expected skipped for disabled+inactive, got %d", preview.Skipped)
	}

	run, err := svc.Materialize(context.Background(), actor, rec.ID, MaterializeRequest{Mode: "full", DryRun: false})
	if err != nil {
		t.Fatal(err)
	}
	if run.Created != 1 {
		t.Fatalf("created want 1 got %d", run.Created)
	}
	children, _, err := repo.ListCompanyChildren(context.Background(), rec.ID, "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 1 || children[0].Status != CompanyStatusNotStarted {
		t.Fatalf("want 1 NotStarted child, got %+v", children)
	}

	again, err := svc.Materialize(context.Background(), actor, rec.ID, MaterializeRequest{Mode: "full", DryRun: false})
	if err != nil {
		t.Fatal(err)
	}
	if again.Exists != 1 || again.Created != 0 {
		t.Fatalf("idempotent exists want 1 created 0 got exists=%d created=%d", again.Exists, again.Created)
	}
}

func TestIsPreSubmitCompanyStatus(t *testing.T) {
	if !IsPreSubmitCompanyStatus("Draft") || !IsPreSubmitCompanyStatus("NotStarted") {
		t.Fatal("Draft/NotStarted should be pre-submit")
	}
	if IsPreSubmitCompanyStatus("PendingReview") {
		t.Fatal("PendingReview is not pre-submit start")
	}
}

func TestMapCompanyStatusForCounts(t *testing.T) {
	if MapCompanyStatusForCounts("NotStarted") != "not_started" {
		t.Fatal()
	}
	if MapCompanyStatusForCounts("Draft") != "not_started" {
		t.Fatal()
	}
	if MapCompanyStatusForCounts("PendingReview") != "pending_review" {
		t.Fatal()
	}
}
