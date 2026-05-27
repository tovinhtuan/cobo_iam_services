package mysql

import (
	"testing"
	"time"
)

func TestDateOnlyUTC_fromMySQLDateTime(t *testing.T) {
	// go-sql-driver/mysql returns DATE columns as time.Time (often with non-midnight clock).
	in := time.Date(2026, 4, 1, 15, 30, 0, 0, time.Local)
	got := dateOnlyUTC(in)
	if got.Format("2006-01-02") != "2026-04-01" {
		t.Fatalf("got %v", got)
	}
	if got.Location() != time.UTC {
		t.Fatalf("location %v", got.Location())
	}
}

func TestDateOnlyUTC_midnightUTC(t *testing.T) {
	in := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	got := dateOnlyUTC(in)
	if !got.Equal(in) {
		t.Fatalf("got %v want %v", got, in)
	}
}
