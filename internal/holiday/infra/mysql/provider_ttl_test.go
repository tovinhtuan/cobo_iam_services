package mysql

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// PER-06: the worker keeps one long-lived provider. Year data and the "calendar exists" check
// are cached for a TTL, so each day check is not a query, yet a calendar uploaded through the
// CMS (in the API process) reaches the worker within the TTL.
func TestDBProvider_TTLRefreshesYear(t *testing.T) {
	db, d := openCaptureDB(t)
	d.queryOK = true
	clock := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	p := NewDBProviderWithTTL(NewRepository(db), 5*time.Minute)
	p.now = func() time.Time { return clock }
	day := time.Date(2027, 1, 4, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if _, _, err := p.IsNonTradingDay(context.Background(), day); err != nil {
			t.Fatal(err)
		}
	}
	if d.dateQueries != 1 {
		t.Fatalf("within TTL want 1 query, got %d", d.dateQueries)
	}
	clock = clock.Add(6 * time.Minute)
	if _, _, err := p.IsNonTradingDay(context.Background(), day); err != nil {
		t.Fatal(err)
	}
	if d.dateQueries != 2 {
		t.Fatalf("after TTL want a reload, got %d queries", d.dateQueries)
	}
}

func TestCompositeProvider_CachesYearCheckForTTL(t *testing.T) {
	db, d := openCaptureDB(t)
	d.queryOK = true
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "2027.json"), []byte(`{"year":2027,"holidays":[{"date":"2027-01-01","type":"PUBLIC_HOLIDAY"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	repo := NewRepository(db)
	c := &CompositeProvider{Repo: repo, DB: NewDBProviderWithTTL(repo, 5*time.Minute), File: disclosureapp.NewHolidayCalendarFileProvider(dir), YearCheckTTL: 5 * time.Minute}
	c.now = func() time.Time { return clock }
	for i := 0; i < 5; i++ {
		off, _, err := c.IsNonTradingDay(context.Background(), time.Date(2027, 1, 1+i, 0, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 && !off {
			t.Fatal("2027-01-01 must come from the file fallback")
		}
	}
	if d.calendarQueries != 1 {
		t.Fatalf("within TTL want 1 calendar check, got %d", d.calendarQueries)
	}
	clock = clock.Add(6 * time.Minute)
	if _, _, err := c.IsNonTradingDay(context.Background(), time.Date(2027, 1, 6, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if d.calendarQueries != 2 {
		t.Fatalf("after TTL want a re-check, got %d", d.calendarQueries)
	}
}
