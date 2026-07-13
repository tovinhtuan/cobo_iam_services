package app

import (
	"math"
	"strings"
	"time"

	"github.com/cobo/cobo_iam_services/internal/personalops/domain"
)

const (
	reasonOnTimeSampleEmpty = "no_completed_items_with_due_and_outcome"
	onTimeSourceCompletedAt = "disclosure_records.completed_at"
)

// IsOutcomeOnTime compares calendar dates in loc: outcome day <= due day.
func IsOutcomeOnTime(outcomeAt time.Time, dueYYYYMMDD string, loc *time.Location) bool {
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

// ComputeOnTimeRate builds the RateMetric contract. Exact only when sample > 0.
func ComputeOnTimeRate(completedOnTime, completedTotal int) domain.RateMetric {
	if completedTotal <= 0 {
		return domain.RateMetric{
			Value:           nil,
			Accuracy:        AccuracyUnavailable,
			Reason:          ptrString(reasonOnTimeSampleEmpty),
			SampleSize:      0,
			CompletedOnTime: 0,
			CompletedTotal:  0,
			Source:          nil,
		}
	}
	pct := math.Round(float64(completedOnTime) * 100 / float64(completedTotal))
	src := onTimeSourceCompletedAt
	return domain.RateMetric{
		Value:           &pct,
		Accuracy:        AccuracyExact,
		Reason:          nil,
		SampleSize:      completedTotal,
		CompletedOnTime: completedOnTime,
		CompletedTotal:  completedTotal,
		Source:          &src,
	}
}
