package app

import "testing"

func TestNormalizePeriodicityFilter(t *testing.T) {
	cases := map[string]string{
		"ad_hoc":       "ad_hoc",
		"irregular":    "ad_hoc",
		"event_based":  "ad_hoc",
		"quarterly":    "quarterly",
		"Hàng quý":     "quarterly",
		"yearly":       "yearly",
		"annual":       "yearly",
		"monthly":      "monthly",
		"daily":        "daily",
		"weekly":       "weekly",
	}
	for in, want := range cases {
		if got := NormalizePeriodicityFilter(in); got != want {
			t.Fatalf("NormalizePeriodicityFilter(%q)=%q want %q", in, got, want)
		}
	}
}

func TestParseTagQuery(t *testing.T) {
	got := ParseTagQuery("Tài chính, Định kỳ, tài chính")
	if len(got) != 2 {
		t.Fatalf("len=%d want 2 (%v)", len(got), got)
	}
}
