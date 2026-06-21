package app

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"
)

// step_key (mig-S1 / Phase 13 STEP_KEY_SPECIFICATION):
// immutable, server-minted, never reused. Format: "stp_" + 12 lowercase Crockford base32 chars.
// step_key is the stable identity of a global workflow step across saves (rename/reorder preserve it;
// delete retires it; re-add mints a new one). It is opaque to all consumers.

// stepKeyAlphabet is lowercase Crockford base32 (no i/l/o/u).
const stepKeyAlphabet = "0123456789abcdefghjkmnpqrstvwxyz"

// MintStepKey returns a new server-minted step_key. Random => never reuses a retired key.
// Never derives identity from array position.
func MintStepKey() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is effectively impossible; degrade to time-seeded uniqueness.
		return fmt.Sprintf("stp_%012d", time.Now().UnixNano()%1e12)
	}
	out := make([]byte, 12)
	for i := range b {
		out[i] = stepKeyAlphabet[int(b[i])%len(stepKeyAlphabet)]
	}
	return "stp_" + string(out)
}

// ResolveStepKey decides the step_key for one incoming step during an upsert, preserving stable
// identity. Precedence (Phase 13A / baseline rules, no auto-positional identity):
//  1. incoming step_key that matches a known existing key  -> preserve (rename/reorder safe)
//  2. incoming step_id that matches an existing step's key -> preserve (legacy clients)
//  3. otherwise                                            -> mint a new key (genuinely new step)
//
// usedKeys guards against two incoming steps resolving to the same key within one save; the second
// gets a freshly minted key so the unique (workflow_id, step_key) constraint never collides.
//
// existingKeys: set of valid server-owned step_keys for the type's current workflow.
// existingKeyByStepID: old step_id -> step_key for that workflow.
func ResolveStepKey(
	step GlobalWorkflowStepInput,
	existingKeys map[string]bool,
	existingKeyByStepID map[string]string,
	usedKeys map[string]bool,
) string {
	key := strings.TrimSpace(step.StepKey)
	if key == "" || !existingKeys[key] {
		// Do not trust a client-supplied key that is not an existing server-owned key.
		key = ""
		if k, ok := existingKeyByStepID[strings.TrimSpace(step.StepID)]; ok {
			key = k
		}
	}
	if key == "" || usedKeys[key] {
		key = MintStepKey()
		// Extremely unlikely collision with an already-used key in this batch: re-mint.
		for usedKeys[key] {
			key = MintStepKey()
		}
	}
	return key
}
