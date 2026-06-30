package timeline

import "strings"

var metadataDenyKeys = map[string]struct{}{
	"password": {}, "token": {}, "secret": {}, "smtp": {}, "ciphertext": {},
	"refresh_token": {}, "access_token": {}, "api_key": {},
}

var metadataAllowKeys = map[string]struct{}{
	"permission_code": {}, "rule_code": {}, "field": {}, "channel": {}, "simulation_mode": {},
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
