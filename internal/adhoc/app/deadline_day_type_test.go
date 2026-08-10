package app

import (
	"net/http"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestEffectiveProposalDeadlineDayType(t *testing.T) {
	cal := ProposalDeadlineDayTypeCalendarDays
	work := ProposalDeadlineDayTypeWorkingDays
	empty := ProposalDeadlineDayType("")

	cases := []struct {
		name string
		in   *ProposalDeadlineDayType
		want ProposalDeadlineDayType
	}{
		{"nil", nil, ProposalDeadlineDayTypeCalendarDays},
		{"empty", &empty, ProposalDeadlineDayTypeCalendarDays},
		{"calendar", &cal, ProposalDeadlineDayTypeCalendarDays},
		{"working", &work, ProposalDeadlineDayTypeWorkingDays},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EffectiveProposalDeadlineDayType(tc.in); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestParseOptionalProposalDeadlineDayType(t *testing.T) {
	got, err := parseOptionalProposalDeadlineDayType("")
	if err != nil || got != nil {
		t.Fatalf("empty: got=%v err=%v", got, err)
	}
	got, err = parseOptionalProposalDeadlineDayType("WORKING_DAYS")
	if err != nil || got == nil || *got != ProposalDeadlineDayTypeWorkingDays {
		t.Fatalf("working: got=%v err=%v", got, err)
	}
	got, err = parseOptionalProposalDeadlineDayType("CALENDAR_DAYS")
	if err != nil || got == nil || *got != ProposalDeadlineDayTypeCalendarDays {
		t.Fatalf("calendar: got=%v err=%v", got, err)
	}
	_, err = parseOptionalProposalDeadlineDayType("working")
	if err == nil {
		t.Fatal("expected invalid enum error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected 400 HTTPError, got %#v", err)
	}
	if he.Details == nil || he.Details["field"] != "proposed_deadline_day_type" {
		t.Fatalf("expected field detail, got %#v", he.Details)
	}
}
