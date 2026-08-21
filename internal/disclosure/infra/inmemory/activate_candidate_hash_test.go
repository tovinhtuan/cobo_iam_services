package inmemory

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func TestActivateTypeVersion_RejectsStaleCandidateHash(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	req := disclosureapp.UpsertTypeVersionRequest{
		Subject:           disclosureapp.Subject{UserID: "u1", CompanyID: "c1"},
		TypeID:            "type-hash",
		Scope:             "global",
		GroupID:           "group-001",
		Name:              "Hash",
		Category:          "periodic",
		TemplateCategory:  "periodic",
		DeadlineStrategy:  "fixed",
		DeadlineRule:      "T+5",
		DisplayGroupCodes: []string{"dg-periodic"},
		Blocks: []disclosureapp.TemplateBlockDTO{{
			BlockKey: "enterprise_workflow", BlockType: "workflow", Enabled: true,
			Config: map[string]any{"steps": []any{
				map[string]any{
					"step_id": "review", "stage": "Review", "department_id": "legal",
					"assignee_role_ids": []any{"reviewer"}, "processing_days": 2, "display_order": 1,
				},
			}},
		}},
	}
	saved, err := r.UpsertTypeVersion(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.ActivateTypeVersion(ctx, disclosureapp.ActivateTypeVersionRequest{
		Subject:               req.Subject,
		TypeID:                saved.TypeID,
		VersionNo:             saved.VersionNo,
		ExpectedCandidateHash: "stale-hash",
	})
	if err == nil {
		t.Fatal("stale candidate hash must be rejected")
	}
}

func TestActivateTypeVersion_EmptyHashUsesLockedCurrent(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	req := disclosureapp.UpsertTypeVersionRequest{
		Subject:           disclosureapp.Subject{UserID: "u1", CompanyID: "c1"},
		TypeID:            "type-hash-empty",
		Scope:             "global",
		GroupID:           "group-001",
		Name:              "Hash",
		Category:          "periodic",
		TemplateCategory:  "periodic",
		DeadlineStrategy:  "fixed",
		DeadlineRule:      "T+5",
		DisplayGroupCodes: []string{"dg-periodic"},
		Blocks: []disclosureapp.TemplateBlockDTO{{
			BlockKey: "enterprise_workflow", BlockType: "workflow", Enabled: true,
			Config: map[string]any{"steps": []any{
				map[string]any{
					"step_id": "review", "stage": "Review", "department_id": "legal",
					"assignee_role_ids": []any{"reviewer"}, "processing_days": 2, "display_order": 1,
				},
			}},
		}},
	}
	saved, err := r.UpsertTypeVersion(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := r.ActivateTypeVersion(ctx, disclosureapp.ActivateTypeVersionRequest{
		Subject:   req.Subject,
		TypeID:    saved.TypeID,
		VersionNo: saved.VersionNo,
	})
	if err != nil {
		t.Fatalf("FE activate without hash must use locked current candidate: %v", err)
	}
	if !resp.IsActive {
		t.Fatal("expected active")
	}
}
