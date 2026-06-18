package app

import (
	"strings"
	"testing"
	"time"
)

func TestCalculateRemainingDays(t *testing.T) {
	loc := reminderCalculatorLocation()
	now := time.Date(2026, 3, 19, 10, 0, 0, 0, loc) // 19/03/2026 10:00 +07

	cases := []struct {
		name string
		due  time.Time
		want int
	}{
		{"due later same day", time.Date(2026, 3, 19, 23, 0, 0, 0, loc), 0},
		{"due earlier same day", time.Date(2026, 3, 19, 1, 0, 0, 0, loc), 0},
		{"due in 5 days", time.Date(2026, 3, 24, 9, 0, 0, 0, loc), 5},
		{"due tomorrow just after midnight", time.Date(2026, 3, 20, 0, 30, 0, 0, loc), 1},
		{"overdue 2 days", time.Date(2026, 3, 17, 9, 0, 0, 0, loc), -2},
		{"zero due returns 0", time.Time{}, 0},
		// UTC instant must be interpreted in Asia/Ho_Chi_Minh: 18:00Z = 01:00 next day +07.
		{"utc instant shifts to next HCM day", time.Date(2026, 3, 19, 18, 0, 0, 0, time.UTC), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := calculateRemainingDays(tc.due, now); got != tc.want {
				t.Fatalf("calculateRemainingDays(%v) = %d, want %d", tc.due, got, tc.want)
			}
		})
	}
}

func TestDetermineUrgencyStatus(t *testing.T) {
	cases := []struct {
		days int
		want string
	}{
		{-10, "Quá hạn"},
		{-1, "Quá hạn"},
		{0, "Đã đến hạn"},
		{1, "Sắp đến hạn"},
		{30, "Sắp đến hạn"},
	}
	for _, tc := range cases {
		if got := determineUrgencyStatus(tc.days); got != tc.want {
			t.Errorf("determineUrgencyStatus(%d) = %q, want %q", tc.days, got, tc.want)
		}
	}
}

func TestExtractStepID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"disc-1:step-2", "step-2"},
		{"step-only", "step-only"},
		{"a:b:c", "c"},
		{"", ""},
		{"trailing:", ""},
	}
	for _, tc := range cases {
		if got := extractStepID(tc.in); got != tc.want {
			t.Errorf("extractStepID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTruncateImplementationGuide(t *testing.T) {
	if got := truncateImplementationGuide("  short  ", 100); got != "short" {
		t.Errorf("trim/short: got %q", got)
	}
	if got := truncateImplementationGuide(strings.Repeat("a", 10), 5); got != "aaaaa..." {
		t.Errorf("truncate: got %q", got)
	}
	exact := strings.Repeat("b", 5)
	if got := truncateImplementationGuide(exact, 5); got != exact {
		t.Errorf("exact length unchanged: got %q", got)
	}
	if got := truncateImplementationGuide("anything", 0); got != "anything" {
		t.Errorf("disabled cap (maxChars<=0): got %q", got)
	}
	// UTF-8 safety: cap counts runes, not bytes; must not split a Vietnamese rune.
	got := truncateImplementationGuide("Báo cáo tài chính", 3)
	if r := []rune(got); len(r) == 0 || r[0] != 'B' || !strings.HasSuffix(got, "...") {
		t.Errorf("utf8 rune-safe truncate: got %q", got)
	}
}
