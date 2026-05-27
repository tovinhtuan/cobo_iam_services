package mysql

import "time"

// dateOnlyUTC normalizes a timestamp to midnight UTC on its calendar date (MySQL DATE semantics).
func dateOnlyUTC(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
