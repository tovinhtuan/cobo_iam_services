package mysql

import "testing"

// TestDecodeGlobalWorkflowManifestSteps_EnvelopeShape covers the shape workflowconfig's real
// Publish() flow writes: {"type_id":..,"workflow_id":..,"version_no":..,"steps":[...]}.
func TestDecodeGlobalWorkflowManifestSteps_EnvelopeShape(t *testing.T) {
	raw := []byte(`{"type_id":"t1","workflow_id":"w1","version_no":2,"steps":[{"step_id":"s1","stage":"Final Step","role":"reviewer","department_id":"d1","processing_days":7,"display_order":1}]}`)
	steps, err := decodeGlobalWorkflowManifestSteps(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("steps=%d want 1", len(steps))
	}
	if steps[0].Role != "reviewer" || steps[0].Stage != "Final Step" {
		t.Fatalf("unexpected step: %+v", steps[0])
	}
}

// TestDecodeGlobalWorkflowManifestSteps_BareArrayShape is the REGRESSION GUARD for the exact bug
// found during Batch R1 implementation: migrations/0101's own v1 backfill wrote
// steps_manifest_json as a bare JSON array (no envelope), confirmed live on DEV for type
// dt-sys-board-resolution. A decoder that only handles the envelope shape would error here and
// GetEffectiveWorkflow would return HTTP 500 for any type still on that legacy-shaped v1 row.
func TestDecodeGlobalWorkflowManifestSteps_BareArrayShape(t *testing.T) {
	raw := []byte(`[{"name":"Họp và thông qua","role":null,"stage":"Họp và thông qua","step_id":"gws-board-res-1","due_rule":"T+3","step_key":"gws-board-res-1","department_id":"Phong","display_order":1,"processing_days":0}]`)
	steps, err := decodeGlobalWorkflowManifestSteps(raw)
	if err != nil {
		t.Fatalf("bare array shape must decode without error, got: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("steps=%d want 1", len(steps))
	}
	if steps[0].Stage != "Họp và thông qua" || steps[0].DueRule != "T+3" {
		t.Fatalf("unexpected step: %+v", steps[0])
	}
	if steps[0].Role != "" {
		t.Fatalf("role should be empty string for JSON null, got %q", steps[0].Role)
	}
}

// TestDecodeGlobalWorkflowManifestSteps_EnvelopeWithEmptySteps ensures an envelope whose "steps"
// key is present but an empty array is NOT mistaken for "no steps key" (which would otherwise
// incorrectly fall through to the bare-array branch and fail).
func TestDecodeGlobalWorkflowManifestSteps_EnvelopeWithEmptySteps(t *testing.T) {
	raw := []byte(`{"type_id":"t1","workflow_id":"w1","version_no":1,"steps":[]}`)
	steps, err := decodeGlobalWorkflowManifestSteps(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 0 {
		t.Fatalf("steps=%d want 0", len(steps))
	}
}

// TestDecodeGlobalWorkflowManifestSteps_Malformed ensures genuinely invalid JSON still errors
// (the dual-shape tolerance must not silently swallow real corruption).
func TestDecodeGlobalWorkflowManifestSteps_Malformed(t *testing.T) {
	raw := []byte(`{not valid json`)
	if _, err := decodeGlobalWorkflowManifestSteps(raw); err == nil {
		t.Fatal("expected an error for malformed JSON, got nil")
	}
}

// TestDecodeGlobalWorkflowManifestSteps_PreservesExplicitDueRule guards the input to the
// due_rule synthesis step in loadActiveGlobalWorkflow ("T+<processing_days>" when due_rule is
// empty): the decoder itself must pass an explicit due_rule through unmodified so the synthesis
// step's "only fill when empty" check has accurate input to test against.
func TestDecodeGlobalWorkflowManifestSteps_PreservesExplicitDueRule(t *testing.T) {
	raw := []byte(`{"steps":[{"step_id":"s1","stage":"X","due_rule":"T+99","processing_days":3,"display_order":1}]}`)
	steps, err := decodeGlobalWorkflowManifestSteps(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("steps=%d want 1", len(steps))
	}
	if steps[0].DueRule != "T+99" {
		t.Fatalf("decoder must not mutate due_rule; got %q", steps[0].DueRule)
	}
}
