package app_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type missingYearProvider struct{}

func (missingYearProvider) IsNonTradingDay(_ context.Context, d time.Time) (bool, string, error) {
	return false, "", fmt.Errorf("missing holiday config for year %d: %w", d.Year(), disclosureapp.ErrHolidayYearMissing)
}

// PER-06: a cycle skipped because the holiday calendar of its year is missing must be logged
// loudly (it was dropped silently, so a working-day template simply stopped producing cycles).
func TestSeedPeriodicCycles_MissingHolidayYearIsLogged(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	repo := &seedApplicableToRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-missing-holiday-year", FrequencyUnit: "monthly", CycleAnchorDay: 5, DeadlineDays: 10,
		}},
		companies: []string{"co-1"},
	}
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{}, disclosureapp.WithHolidayCalendarProvider(missingYearProvider{}))
	n, err := svc.SeedPeriodicCycles(context.Background(), time.Date(2027, 1, 10, 3, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("cycle must be skipped without a calendar, seeded=%d", n)
	}
	out := buf.String()
	if !strings.Contains(out, `"level":"WARN"`) || !strings.Contains(out, `"reason":"holiday_year_missing"`) || !strings.Contains(out, "t-missing-holiday-year") {
		t.Fatalf("expected WARN reason=holiday_year_missing with type_id, got: %s", out)
	}
}
