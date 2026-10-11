package workflow

import (
	"testing"
	"time"
)

// DLA-01 (same pattern): the published date is the Asia/Ho_Chi_Minh date of the approval.
func TestPublishedDateIsHCMDate(t *testing.T) {
	if got := publishedDate(time.Date(2026, 10, 19, 23, 0, 0, 0, time.UTC)); got != "2026-10-20" {
		t.Fatalf("approval at 20/10 06:00 HCM recorded as %s", got)
	}
	if got := publishedDate(time.Date(2026, 10, 19, 16, 0, 0, 0, time.UTC)); got != "2026-10-19" {
		t.Fatalf("approval at 19/10 23:00 HCM recorded as %s", got)
	}
}
