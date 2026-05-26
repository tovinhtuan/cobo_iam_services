package app

import (
	"testing"
	"time"
)

func TestAlertStatusFromRemainingDays(t *testing.T) {
	tests := []struct {
		days int
		want string
	}{
		{-1, "OVERDUE"},
		{0, "DUE_SOON"},
		{3, "UPCOMING"},
	}
	for _, tc := range tests {
		if got := alertStatusFromRemainingDays(tc.days, false); got != tc.want {
			t.Fatalf("days=%d got %s want %s", tc.days, got, tc.want)
		}
	}
}

func TestMatchesDateRange(t *testing.T) {
	if !matchesDateRange("2026-05-20", "2026-05-01", "2026-05-31") {
		t.Fatal("expected in range")
	}
	if matchesDateRange("2026-05-20", "2026-06-01", "") {
		t.Fatal("expected out of range")
	}
}

func TestIsDraftRecordStatus_excluded(t *testing.T) {
	if !isDraftRecordStatus("draft") {
		t.Fatal("expected draft")
	}
}

func TestIsTerminalRecordStatus_includesPublished(t *testing.T) {
	if !isTerminalRecordStatus("Published") {
		t.Fatal("expected published terminal")
	}
}

func TestNormalizeStatusFilter_pendingConfirm(t *testing.T) {
	if got := normalizeStatusFilter("pending_confirm"); got != "PENDING_CONFIRM" {
		t.Fatalf("got %s", got)
	}
}

func TestRemainingDaysFromDue(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, loc)
	if got := remainingDaysFromDue("2026-05-27", now, loc); got != 2 {
		t.Fatalf("got %d want 2", got)
	}
}
