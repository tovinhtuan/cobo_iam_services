package app

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type fixedID struct{ n int }

func (f *fixedID) NewID(prefix string) string {
	f.n++
	return prefix + "_" + strconv.Itoa(f.n)
}

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func allowAll(_ context.Context, _, _ string, _ ...string) error { return nil }

func denyPublish(_ context.Context, _, _ string, perms ...string) error {
	for _, p := range perms {
		if p == PermRecordPublish || p == PermRecordMaterialize {
			return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "denied", nil)
		}
	}
	return nil
}

func TestValidateCycleKey(t *testing.T) {
	ok := []string{"daily:2026-09-24", "weekly:2026-W39", "monthly:2026-09", "quarterly:2026-Q3", "yearly:2026"}
	for _, k := range ok {
		if !ValidateCycleKey(k) {
			t.Fatalf("expected valid: %s", k)
		}
	}
	bad := []string{"", "2026-Q3", "quarterly:2026-Q5", "event:x", "daily:2026-09-24#archived:1"}
	for _, k := range bad {
		if ValidateCycleKey(k) {
			t.Fatalf("expected invalid: %s", k)
		}
	}
}

func TestGlobalRecordLifecycle_DirectPublish(t *testing.T) {
	repo := NewMemoryRepository()
	repo.SeedTemplate(TemplateInfo{TypeID: "tpl-bctc", Scope: "global", ActiveVersionNo: 2, PortalState: "active", Periodicity: "quarterly"})
	svc := NewGlobalRecordsService(repo, &fixedID{}, fixedClock{t: time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)}, allowAll)
	actor := Actor{UserID: "u1", MembershipID: "m1", CompanyID: "c_001"}

	rec, err := svc.Create(context.Background(), actor, "tpl-bctc", "BCTC Q3", "sum", "content body", "quarterly:2026-Q3")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if rec.Status != StatusDraft {
		t.Fatalf("status=%s", rec.Status)
	}

	// Duplicate cycle
	if _, err := svc.Create(context.Background(), actor, "tpl-bctc", "dup", "s", "c", "quarterly:2026-Q3"); err == nil {
		t.Fatal("expected duplicate conflict")
	}

	published, err := svc.Publish(context.Background(), actor, rec.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.Status != StatusPublished {
		t.Fatalf("status=%s", published.Status)
	}
	if published.TemplateVersionNo == nil || *published.TemplateVersionNo != 2 {
		t.Fatalf("version pin missing")
	}

	// Idempotent publish
	again, err := svc.Publish(context.Background(), actor, rec.ID)
	if err != nil || again.Status != StatusPublished {
		t.Fatalf("idempotent publish failed: %v", err)
	}

	if _, err := svc.Update(context.Background(), actor, rec.ID, "x", "y", "z"); err == nil {
		t.Fatal("expected update conflict on Published")
	}

	mat, err := svc.Materialize(context.Background(), actor, rec.ID, MaterializeRequest{Mode: "full", DryRun: false})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if mat.Created != 2 {
		t.Fatalf("created=%d want 2", mat.Created)
	}
	mat2, err := svc.Materialize(context.Background(), actor, rec.ID, MaterializeRequest{Mode: "incremental", DryRun: false})
	if err != nil {
		t.Fatalf("materialize2: %v", err)
	}
	if mat2.Exists != 2 || mat2.Created != 0 {
		t.Fatalf("idempotent materialize exists=%d created=%d", mat2.Exists, mat2.Created)
	}

	archived, err := svc.Archive(context.Background(), actor, rec.ID)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if archived.Status != StatusArchived {
		t.Fatalf("status=%s", archived.Status)
	}
	if _, err := svc.Materialize(context.Background(), actor, rec.ID, MaterializeRequest{Mode: "full"}); err == nil {
		t.Fatal("expected materialize blocked after archive")
	}

	// New draft same business cycle after archive frees unique
	rec2, err := svc.Create(context.Background(), actor, "tpl-bctc", "BCTC Q3 v2", "s", "c", "quarterly:2026-Q3")
	if err != nil {
		t.Fatalf("recreate after archive: %v", err)
	}
	if rec2.Status != StatusDraft {
		t.Fatalf("status=%s", rec2.Status)
	}
}

func TestPublishDeniedWithoutPermission(t *testing.T) {
	repo := NewMemoryRepository()
	repo.SeedTemplate(TemplateInfo{TypeID: "tpl", Scope: "global", ActiveVersionNo: 1, PortalState: "active"})
	svc := NewGlobalRecordsService(repo, &fixedID{}, fixedClock{t: time.Now().UTC()}, denyPublish)
	actor := Actor{UserID: "u1", MembershipID: "m1"}
	rec, err := svc.Create(context.Background(), actor, "tpl", "t", "s", "c", "yearly:2026")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Publish(context.Background(), actor, rec.ID); err == nil {
		t.Fatal("expected publish denied")
	}
}

func TestMaterializeDraftRejected(t *testing.T) {
	repo := NewMemoryRepository()
	repo.SeedTemplate(TemplateInfo{TypeID: "tpl", Scope: "global", ActiveVersionNo: 1, PortalState: "active"})
	svc := NewGlobalRecordsService(repo, &fixedID{}, fixedClock{t: time.Now().UTC()}, allowAll)
	actor := Actor{UserID: "u1", MembershipID: "m1"}
	rec, _ := svc.Create(context.Background(), actor, "tpl", "t", "s", "c", "monthly:2026-09")
	if _, err := svc.Materialize(context.Background(), actor, rec.ID, MaterializeRequest{Mode: "full"}); err == nil {
		t.Fatal("expected reject draft materialize")
	}
}
