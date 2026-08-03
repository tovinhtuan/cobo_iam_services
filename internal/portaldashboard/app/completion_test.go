package app

import (
	"testing"
	"time"
)

func TestIsOutcomeOnTime(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	due := "2026-08-01"
	if !isOutcomeOnTime(time.Date(2026, 7, 31, 12, 0, 0, 0, loc), due, loc) {
		t.Fatal("before due should be on time")
	}
	if !isOutcomeOnTime(time.Date(2026, 8, 1, 23, 59, 0, 0, loc), due, loc) {
		t.Fatal("same calendar day should be on time")
	}
	if isOutcomeOnTime(time.Date(2026, 8, 2, 0, 0, 0, 0, loc), due, loc) {
		t.Fatal("after due should be late")
	}
	if isOutcomeOnTime(time.Now(), "", loc) {
		t.Fatal("empty due")
	}
}
