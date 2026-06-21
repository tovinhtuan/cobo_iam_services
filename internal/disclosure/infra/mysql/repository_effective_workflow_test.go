package mysql

import (
	"os"
	"strings"
	"testing"
)

// TestGetEffectiveWorkflow_BranchOrder_OverrideThenGlobalWorkflowThenLegacy guards the precedence
// mandated by ADR_WORKFLOW_DATA_SOURCE_ALIGNMENT.md ("1. company override, 2. active
// global_workflow_versions, 3. legacy enterprise_workflow block, 4. empty") — company override
// must be checked (and able to return) before loadActiveGlobalWorkflow is called, which in turn
// must be called before the legacy ExtractTemplateWorkflow fallback.
func TestGetEffectiveWorkflow_BranchOrder_OverrideThenGlobalWorkflowThenLegacy(t *testing.T) {
	src := readGlobalWorkflowReadTestSrc(t, "repository.go")
	fn := extractFunc(t, src, "func (r *Repository) GetEffectiveWorkflow")

	overrideIdx := strings.Index(fn, "view.ActiveVersion != nil")
	globalIdx := strings.Index(fn, "r.loadActiveGlobalWorkflow(ctx, typeID)")
	legacyIdx := strings.Index(fn, "ExtractTemplateWorkflow(detail.Blocks)")

	if overrideIdx == -1 {
		t.Fatal("GetEffectiveWorkflow must still check view.ActiveVersion (company override) — this branch must remain untouched per ADR")
	}
	if globalIdx == -1 {
		t.Fatal("GetEffectiveWorkflow must call loadActiveGlobalWorkflow (Batch R1 fix) before falling back to legacy")
	}
	if legacyIdx == -1 {
		t.Fatal("GetEffectiveWorkflow must still fall back to ExtractTemplateWorkflow when no override and no active global workflow exist")
	}
	if !(overrideIdx < globalIdx && globalIdx < legacyIdx) {
		t.Fatalf("branch order must be override(%d) < global_workflow(%d) < legacy(%d)", overrideIdx, globalIdx, legacyIdx)
	}
}

// TestGetEffectiveWorkflow_GlobalWorkflowSource_SetExactlyOnHit ensures the "global_workflow"
// source string is only assigned inside the new branch (not the override or legacy branches),
// preserving the three distinct source values the ADR specifies.
func TestGetEffectiveWorkflow_GlobalWorkflowSourceLiteral_Present(t *testing.T) {
	src := readGlobalWorkflowReadTestSrc(t, "repository.go")
	fn := extractFunc(t, src, "func (r *Repository) GetEffectiveWorkflow")
	if !strings.Contains(fn, `dto.Source = "global_workflow"`) {
		t.Fatal(`GetEffectiveWorkflow must set dto.Source = "global_workflow" when loadActiveGlobalWorkflow finds an active version`)
	}
}

// TestLoadActiveGlobalWorkflow_ReadsActiveVersionNo_NotPublishedVersionNo is the mechanism that
// guarantees Publish-without-Activate has zero effect on the runtime (Publish only ever updates
// global_workflows.published_version_no; Activate updates active_version_no). The query must
// join on w.active_version_no and must NOT reference published_version_no at all — otherwise a
// Publish alone could leak into the runtime-facing response, reproducing the exact regression
// Phase 0 (PHASE_0_CURRENT_FAILURE_PROOF.md) proved for the OLD code path, just via a new one.
func TestLoadActiveGlobalWorkflow_ReadsActiveVersionNo_NotPublishedVersionNo(t *testing.T) {
	src := readGlobalWorkflowReadTestSrc(t, "global_workflow_read.go")
	fn := extractFunc(t, src, "func (r *Repository) loadActiveGlobalWorkflow")

	if !strings.Contains(fn, "w.active_version_no") {
		t.Fatal("loadActiveGlobalWorkflow must join on global_workflows.active_version_no")
	}
	if strings.Contains(fn, "published_version_no") {
		t.Fatal("loadActiveGlobalWorkflow must NOT reference published_version_no — reading it would let a Publish-without-Activate leak into the runtime response, breaking publish≠activate semantics")
	}
}

// TestLoadActiveGlobalWorkflow_UsesStatusActiveGuard mirrors the existing global_workflows.status
// guard already used elsewhere in this package (e.g. GetGlobalWorkflow, CountGlobalWorkflowsByTypeId)
// — a soft-deleted/archived global_workflows row must not be treated as a live source.
func TestLoadActiveGlobalWorkflow_UsesStatusActiveGuard(t *testing.T) {
	src := readGlobalWorkflowReadTestSrc(t, "global_workflow_read.go")
	fn := extractFunc(t, src, "func (r *Repository) loadActiveGlobalWorkflow")
	if !strings.Contains(fn, "w.status = 'active'") {
		t.Fatal("loadActiveGlobalWorkflow must filter on global_workflows.status = 'active'")
	}
}

func readGlobalWorkflowReadTestSrc(t *testing.T, file string) string {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	// Source files in this repo may have CRLF line endings (Windows checkout) — normalize so
	// "\n}\n"-style boundary checks below work regardless of the checked-out line-ending style.
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// extractFunc returns the source text of the function starting at the first occurrence of
// signaturePrefix, up to (and including) the matching closing brace at column 0 (Go's standard
// top-level function closing-brace convention). Fails the test if not found.
func extractFunc(t *testing.T, src, signaturePrefix string) string {
	t.Helper()
	start := strings.Index(src, signaturePrefix)
	if start == -1 {
		t.Fatalf("function with signature prefix %q not found", signaturePrefix)
	}
	rest := src[start:]
	end := strings.Index(rest, "\n}\n")
	if end == -1 {
		t.Fatalf("could not find end of function %q", signaturePrefix)
	}
	return rest[:end]
}
