package app

import (
	"net/http"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestResolveProposedDeadline_daysFromLegacyField(t *testing.T) {
	days, date, err := resolveProposedDeadline("2026-05-24", "20", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if date != nil {
		t.Fatalf("expected nil calendar date, got %v", *date)
	}
	if days == nil || *days != 20 {
		t.Fatalf("expected 20 days, got %#v", days)
	}
}

func TestResolveProposedDeadline_explicitDays(t *testing.T) {
	days, date, err := resolveProposedDeadline("", "", 15)
	if err != nil || date != nil || days == nil || *days != 15 {
		t.Fatalf("got days=%#v date=%#v err=%v", days, date, err)
	}
}

func TestResolveProposedDeadline_calendarDate(t *testing.T) {
	days, date, err := resolveProposedDeadline("", "2026-06-01", 0)
	if err != nil || days != nil || date == nil || *date != "2026-06-01" {
		t.Fatalf("got days=%#v date=%#v err=%v", days, date, err)
	}
}

func TestResolveProposedDeadline_invalid(t *testing.T) {
	_, _, err := resolveProposedDeadline("", "not-a-date", 0)
	httpErr, ok := err.(*perr.HTTPError)
	if !ok || httpErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected 400, got %#v", err)
	}
}
