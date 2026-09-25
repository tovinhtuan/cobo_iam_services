package workflowdept

import "strings"

// SnapshotCatalogTokens returns distinct non-empty department tokens from a snapshot.
// Callers must still confirm each token exists in the catalog before registry insert.
func SnapshotCatalogTokens(departments []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(departments))
	for _, raw := range departments {
		token := strings.TrimSpace(raw)
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}
