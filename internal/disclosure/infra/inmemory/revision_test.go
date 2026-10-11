package inmemory_test

import (
	"context"
	"net/http"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func withCandidate(t *testing.T, req disclosureapp.UpsertTypeVersionRequest) disclosureapp.UpsertTypeVersionRequest {
	t.Helper()
	c, err := disclosureapp.BuildTemplatePublicationCandidate(req)
	if err != nil {
		t.Fatalf("build candidate: %v", err)
	}
	req.PublicationCandidate = &c
	return req
}

func headHash(t *testing.T, repo *inmemory.Repository, typeID string) string {
	t.Helper()
	items, err := repo.ListTypeVersions(context.Background(), "c1", typeID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	for _, it := range items {
		if !it.IsActive && !it.IsReleased {
			return it.PublicationCandidateHash
		}
	}
	t.Fatal("no open draft")
	return ""
}

func TestUpsertTypeVersion_ExpectedRevisionCompareAndSet(t *testing.T) {
	repo := inmemory.NewRepository()
	ctx := context.Background()
	first := withCandidate(t, baseUpsert("type-rev"))
	if _, err := repo.UpsertTypeVersion(ctx, first); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	loaded := disclosureapp.RevisionFromCandidateHash(headHash(t, repo, "type-rev"))
	if loaded == "" {
		t.Fatal("open draft must expose a candidate hash")
	}

	// Admin A saves with the revision they loaded → OK, content changes.
	a := baseUpsert("type-rev")
	a.Name = "Edited by A"
	a.ExpectedRevision = loaded
	if _, err := repo.UpsertTypeVersion(ctx, withCandidate(t, a)); err != nil {
		t.Fatalf("A save with fresh revision: %v", err)
	}

	// Admin B still holds the old revision → 409 STALE_ETAG, draft untouched.
	b := baseUpsert("type-rev")
	b.Name = "Edited by B"
	b.ExpectedRevision = loaded
	_, err := repo.UpsertTypeVersion(ctx, withCandidate(t, b))
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusConflict || he.Code != perr.CodeStaleEtag {
		t.Fatalf("want 409 STALE_ETAG, got %v", err)
	}
	if headHash(t, repo, "type-rev") == disclosureapp.CandidateHashFromRevision(loaded) {
		t.Fatal("draft must still hold A's content")
	}

	// Legacy client without expected_revision keeps last-write-wins.
	legacy := baseUpsert("type-rev")
	legacy.Name = "Legacy"
	if _, err := repo.UpsertTypeVersion(ctx, withCandidate(t, legacy)); err != nil {
		t.Fatalf("legacy save: %v", err)
	}
}

func TestUpsertTypeVersion_ExpectedRevisionWithoutTargetIsStale(t *testing.T) {
	repo := inmemory.NewRepository()
	req := baseUpsert("type-missing")
	req.ExpectedRevision = "pc1:deadbeef"
	_, err := repo.UpsertTypeVersion(context.Background(), withCandidate(t, req))
	if he, ok := perr.AsHTTPError(err); !ok || he.Code != perr.CodeStaleEtag {
		t.Fatalf("want STALE_ETAG for missing write target, got %v", err)
	}
}
