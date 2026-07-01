package timeline

import "strings"

var metadataDenyKeys = map[string]struct{}{
	"password": {}, "token": {}, "secret": {}, "smtp": {}, "ciphertext": {},
	"refresh_token": {}, "access_token": {}, "api_key": {},
}

var metadataAllowKeys = map[string]struct{}{
	"permission_code": {}, "rule_code": {}, "field": {}, "channel": {}, "simulation_mode": {},
	"approval_id": {}, "aggregate_type": {}, "change_type": {}, "version_no": {},
	"base_live_version_no": {}, "status": {},
	"delegation_id": {}, "scope_type": {}, "scope_id": {},
	"delegatee_membership_id": {}, "delegator_membership_id": {}, "permission_set": {},
	"session_id": {}, "target_membership_id": {}, "requester_membership_id": {},
	"capability_set": {}, "capability": {}, "expires_at": {}, "reason": {}, "approval_step": {},
}

// SanitizeMetadata returns a safe subset of audit metadata for timeline presentation.
func SanitizeMetadata(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any)
	for k, v := range in {
		key := strings.ToLower(strings.TrimSpace(k))
		if _, deny := metadataDenyKeys[key]; deny {
			continue
		}
		if _, allow := metadataAllowKeys[key]; !allow {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
