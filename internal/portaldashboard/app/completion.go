package app

import (
	"context"
	"math"
	"strings"
	"time"

	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
)

const (
	SourceDisclosureCompletedAt = "disclosure_records.completed_at"
	ReasonCompletionSourceUnavailable = "completion_source_unavailable"
)

// CompletedAtReader loads disclosure_records.completed_at for record IDs (read-only).
type CompletedAtReader interface {
	MapCompletedAt(ctx context.Context, companyID string, recordIDs []string) (map[string]time.Time, error)
}

type completionFetch struct {
	ok               bool
	err              error
	completedOnTime  int
	completedTotal   int
	completedOverdue int
}

// isOutcomeOnTime mirrors personalops calendar-day rule: outcome day <= due day in loc.
func isOutcomeOnTime(outcomeAt time.Time, dueYYYYMMDD string, loc *time.Location) bool {
	dueYYYYMMDD = strings.TrimSpace(dueYYYYMMDD)
	if dueYYYYMMDD == "" || loc == nil {
		return false
	}
	due, err := time.ParseInLocation("2006-01-02", dueYYYYMMDD, loc)
	if err != nil {
		return false
	}
	outcomeAt = outcomeAt.In(loc)
	outcomeDay := time.Date(outcomeAt.Year(), outcomeAt.Month(), outcomeAt.Day(), 0, 0, 0, 0, loc)
	dueDay := time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, loc)
	return !outcomeDay.After(dueDay)
}

func computeCompletionFromAlerts(
	alerts []deadlinealertsapp.DeadlineAlertDTO,
	completedAt map[string]time.Time,
	loc *time.Location,
) completionFetch {
	seen := map[string]struct{}{}
	onTime := 0
	total := 0
	overdue := 0
	for _, a := range alerts {
		id := strings.TrimSpace(a.RecordID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		due := strings.TrimSpace(a.DueDate)
		if due == "" {
			continue
		}
		ca, ok := completedAt[id]
		if !ok {
			continue
		}
		total++
		if isOutcomeOnTime(ca, due, loc) {
			onTime++
		} else {
			overdue++
		}
	}
	return completionFetch{
		ok:               true,
		completedOnTime:  onTime,
		completedTotal:   total,
		completedOverdue: overdue,
	}
}

func onTimeRatePercent(onTime, total int) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round(float64(onTime) * 100 / float64(total))
}

func intPtr(v int) *int { return &v }
