package app_test

import (
	"testing"
	"time"

	personalopsapp "github.com/cobo/cobo_iam_services/internal/personalops/app"
)

// DLA-01: personal ops ("Việc của tôi") counts days from the Asia/Ho_Chi_Minh date of now.
func TestRemainingDays_UTCInstantEarlyHCM(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	now := time.Date(2026, 10, 19, 18, 30, 0, 0, time.UTC) // 20/10 01:30 HCM
	if got := personalopsapp.RemainingDays("2026-10-20", now, loc); got != 0 {
		t.Fatalf("due today in HCM: got %d want 0", got)
	}
	if got := personalopsapp.RemainingDays("2026-10-19", now, loc); got != -1 {
		t.Fatalf("due yesterday in HCM: got %d want -1", got)
	}
}
