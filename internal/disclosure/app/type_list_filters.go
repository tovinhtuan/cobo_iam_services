package app

import "strings"

// NormalizePeriodicityFilter maps user/API frequency keys to canonical filter keys.
func NormalizePeriodicityFilter(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "ad_hoc", "adhoc", "irregular", "event_based", "event-based", "bat_thuong", "bất thường":
		return "ad_hoc"
	case "daily", "hang_ngay", "hàng ngày":
		return "daily"
	case "weekly", "hang_tuan", "hàng tuần":
		return "weekly"
	case "monthly", "hang_thang", "hàng tháng":
		return "monthly"
	case "quarterly", "hang_quy", "hàng quý":
		return "quarterly"
	case "yearly", "annual", "hang_nam", "hàng năm":
		return "yearly"
	default:
		return s
	}
}

// DefaultFrequencyFilterOptions is the catalog of frequency filters for portal UI.
func DefaultFrequencyFilterOptions() []FrequencyFilterOptionDTO {
	return []FrequencyFilterOptionDTO{
		{Value: "ad_hoc", Label: "Bất thường"},
		{Value: "daily", Label: "Hàng ngày"},
		{Value: "weekly", Label: "Hàng tuần"},
		{Value: "monthly", Label: "Hàng tháng"},
		{Value: "quarterly", Label: "Hàng quý"},
		{Value: "yearly", Label: "Hàng năm"},
	}
}

// ParseTagQuery splits comma-separated tag filter values.
func ParseTagQuery(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, t)
	}
	return out
}
