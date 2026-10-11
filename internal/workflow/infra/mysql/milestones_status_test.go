package mysql

import (
	"database/sql"
	"testing"
)

func ns(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

// RMC-06: the reminders API must reflect the occurrence outcome, not just "seeded".
func TestReminderStatusFromRow(t *testing.T) {
	cases := []struct {
		name   string
		sent   int64
		occID  sql.NullString
		status sql.NullString
		code   sql.NullString
		want   string
	}{
		{"not seeded", 0, ns(""), ns(""), ns(""), "pending"},
		{"obsolete (no occurrence)", 1, ns(""), ns(""), ns(""), "cancelled"},
		{"queued", 1, ns("o1"), ns("PENDING"), ns(""), "pending"},
		{"retrying", 1, ns("o1"), ns("RETRY_SCHEDULED"), ns(""), "pending"},
		{"dispatching", 1, ns("o1"), ns("DISPATCHING"), ns(""), "pending"},
		{"sent", 1, ns("o1"), ns("SENT"), ns(""), "sent"},
		{"expired", 1, ns("o1"), ns("FAILED"), ns("REMINDER_EXPIRED"), "cancelled"},
		{"company inactive", 1, ns("o1"), ns("FAILED"), ns("COMPANY_INACTIVE"), "cancelled"},
		{"obsolete at dispatch", 1, ns("o1"), ns("FAILED"), ns("REMINDER_OBSOLETE"), "cancelled"},
		{"failed", 1, ns("o1"), ns("FAILED"), ns("EMAIL_PROVIDER_PERMANENT_ERROR"), "failed"},
		{"tolerates legacy value 2", 2, ns(""), ns(""), ns(""), "cancelled"},
		{"seeded but occurrence row missing", 1, ns("o1"), ns(""), ns(""), "pending"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := reminderStatusFromRow(tc.sent, tc.occID, tc.status, tc.code); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
