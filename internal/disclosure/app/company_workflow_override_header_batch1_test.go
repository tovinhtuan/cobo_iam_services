package app

import (
	"encoding/json"
	"testing"
	"time"
)

// TestCompanyWorkflowOverrideHeaderDTO_Batch1Fields_SerializationRoundTrip is the Sprint 3 /
// Batch 1 (Workflow Override Metadata Foundation) regression guard: proves the 6 new fields
// added to CompanyWorkflowOverrideHeaderDTO round-trip correctly through JSON, both when fully
// populated (the "determinable" case) and when at their zero/omitted values (the "unknown" case
// — the default for every row backfilled by migration 0103). No DB involved; this is a pure
// struct/serialization test, matching this batch's deliberately narrow scope (no repository
// wiring — see docs/ai-cache/workflow-override-foundation-batch1/PREFLIGHT_AUDIT.md).
func TestCompanyWorkflowOverrideHeaderDTO_Batch1Fields_SerializationRoundTrip(t *testing.T) {
	t.Run("determinable base (global_workflow)", func(t *testing.T) {
		versionNo := 2
		checkedAt := time.Date(2026, 6, 22, 14, 0, 0, 0, time.UTC)
		original := CompanyWorkflowOverrideHeaderDTO{
			OverrideID:        "ovr_test_1",
			TypeID:            "bao-cao-tai-chinh-quy-1",
			CompanyID:         "144ca32b-cd59-467b-aba9-9576d3b148ad",
			Status:            "approved",
			ActiveVersionNo:   5,
			UpdatedAt:         checkedAt,
			BaseSource:        "global_workflow",
			BaseWorkflowID:    "019e9237-61d6-7180-9702-f9a1e0553540",
			BaseVersionNo:     &versionNo,
			BaseHash:          "",
			StaleStatus:       "unknown",
			LastRebaseCheckAt: nil,
		}

		raw, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var decoded CompanyWorkflowOverrideHeaderDTO
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if decoded.BaseSource != "global_workflow" {
			t.Errorf("BaseSource = %q, want %q", decoded.BaseSource, "global_workflow")
		}
		if decoded.BaseWorkflowID != original.BaseWorkflowID {
			t.Errorf("BaseWorkflowID = %q, want %q", decoded.BaseWorkflowID, original.BaseWorkflowID)
		}
		if decoded.BaseVersionNo == nil || *decoded.BaseVersionNo != versionNo {
			t.Errorf("BaseVersionNo = %v, want %d", decoded.BaseVersionNo, versionNo)
		}
		if decoded.StaleStatus != "unknown" {
			t.Errorf("StaleStatus = %q, want %q", decoded.StaleStatus, "unknown")
		}
	})

	t.Run("unknown base (the Batch 1 default for every backfilled row)", func(t *testing.T) {
		original := CompanyWorkflowOverrideHeaderDTO{
			OverrideID:      "ovr_test_2",
			TypeID:          "dt-event-major-change",
			CompanyID:       "c_001",
			Status:          "approved",
			ActiveVersionNo: 1,
			BaseSource:      "unknown",
			StaleStatus:     "unknown",
			// BaseWorkflowID, BaseVersionNo, BaseHash, LastRebaseCheckAt all left at zero value —
			// exactly matching what migration 0103 leaves for every row with no determinable base.
		}

		raw, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var decoded CompanyWorkflowOverrideHeaderDTO
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if decoded.BaseSource != "unknown" {
			t.Errorf("BaseSource = %q, want %q", decoded.BaseSource, "unknown")
		}
		if decoded.BaseVersionNo != nil {
			t.Errorf("BaseVersionNo = %v, want nil", decoded.BaseVersionNo)
		}
		if decoded.BaseWorkflowID != "" {
			t.Errorf("BaseWorkflowID = %q, want empty", decoded.BaseWorkflowID)
		}
		if decoded.LastRebaseCheckAt != nil {
			t.Errorf("LastRebaseCheckAt = %v, want nil", decoded.LastRebaseCheckAt)
		}
		// omitempty fields must not appear in the wire JSON at all when zero-valued — confirms
		// Batch-1-untouched rows don't leak noisy empty fields to any future consumer.
		var asMap map[string]any
		if err := json.Unmarshal(raw, &asMap); err != nil {
			t.Fatalf("Unmarshal to map: %v", err)
		}
		for _, omittedKey := range []string{"base_workflow_id", "base_version_no", "base_hash", "last_rebase_check_at"} {
			if _, present := asMap[omittedKey]; present {
				t.Errorf("expected %q to be omitted from JSON when zero-valued, but it was present", omittedKey)
			}
		}
	})
}
