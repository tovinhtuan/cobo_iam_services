package configversion

import (
	"encoding/json"
	"sort"
)

// CompareSummary is a safe structural diff between two JSON snapshots (read-only).
type CompareSummary struct {
	FromVersionNo int            `json:"from_version_no"`
	ToVersionNo   int            `json:"to_version_no"`
	ChangedKeys   []string       `json:"changed_keys"`
	FromSize      int            `json:"from_size_bytes"`
	ToSize         int            `json:"to_size_bytes"`
	Equal          bool           `json:"equal"`
	Details        map[string]any `json:"details,omitempty"`
}

// CompareJSON performs a minimal structural comparison without exposing secrets.
func CompareJSON(from, to []byte, fromVer, toVer int) (*CompareSummary, error) {
	var fromObj, toObj map[string]any
	if len(from) > 0 {
		if err := json.Unmarshal(from, &fromObj); err != nil {
			return nil, err
		}
	}
	if len(to) > 0 {
		if err := json.Unmarshal(to, &toObj); err != nil {
			return nil, err
		}
	}
	changed := diffTopLevelKeys(fromObj, toObj)
	sort.Strings(changed)
	equal := len(changed) == 0 && len(from) == len(to)
	if equal && len(from) > 0 {
		equal = string(from) == string(to)
	}
	return &CompareSummary{
		FromVersionNo: fromVer,
		ToVersionNo:   toVer,
		ChangedKeys:   changed,
		FromSize:      len(from),
		ToSize:        len(to),
		Equal:         equal,
		Details: map[string]any{
			"changed_key_count": len(changed),
		},
	}, nil
}

func diffTopLevelKeys(a, b map[string]any) []string {
	keys := map[string]struct{}{}
	for k := range a {
		keys[k] = struct{}{}
	}
	for k := range b {
		keys[k] = struct{}{}
	}
	out := make([]string, 0, len(keys))
	for k := range keys {
		if !jsonEqual(a[k], b[k]) {
			out = append(out, k)
		}
	}
	return out
}

func jsonEqual(a, b any) bool {
	ja, err1 := json.Marshal(a)
	jb, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return string(ja) == string(jb)
}
