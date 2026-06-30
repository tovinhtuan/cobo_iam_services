package conflict

import "sort"

var severityRank = map[string]int{
	SeverityBlocking: 0,
	SeverityWarning:  1,
	SeverityInfo:     2,
}

// MergeAndSort deduplicates by code+resource_type+resource_id (keeps highest severity) and sorts.
func MergeAndSort(results []Result) []Result {
	type key struct {
		code         string
		resourceType string
		resourceID   string
	}
	best := make(map[key]Result, len(results))
	for _, r := range results {
		k := key{code: r.Code, resourceType: r.ResourceType, resourceID: r.ResourceID}
		if prev, ok := best[k]; !ok || severityRank[r.Severity] < severityRank[prev.Severity] {
			best[k] = r
		}
	}
	out := make([]Result, 0, len(best))
	for _, r := range best {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		ri, rj := severityRank[out[i].Severity], severityRank[out[j].Severity]
		if ri != rj {
			return ri < rj
		}
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		if out[i].ResourceID == "" && out[j].ResourceID != "" {
			return false
		}
		if out[i].ResourceID != "" && out[j].ResourceID == "" {
			return true
		}
		return out[i].ResourceID < out[j].ResourceID
	})
	return out
}
