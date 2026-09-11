package app

import (
	"strings"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// TimelinessStatus is the BE-owned workflow/step processing timeliness enum.
// It is derived (never persisted) and independent from disclosure DueAt.
type TimelinessStatus string

const (
	TimelinessWithinDeadline TimelinessStatus = "WITHIN_DEADLINE"
	TimelinessOnTime         TimelinessStatus = "ON_TIME"
	TimelinessOverdue        TimelinessStatus = "OVERDUE"
	TimelinessCompletedLate  TimelinessStatus = "COMPLETED_LATE"
)

// HCMDate returns the Asia/Ho_Chi_Minh calendar date (00:00) for t.
func HCMDate(t time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = disclosureapp.CompanyLocation("")
	}
	local := t.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
}

// ParseHCMBusinessDate parses YYYY-MM-DD as an HCM calendar date.
// Empty or invalid input returns ok=false.
func ParseHCMBusinessDate(raw string, loc *time.Location) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if loc == nil {
		loc = disclosureapp.CompanyLocation("")
	}
	d, err := time.ParseInLocation("2006-01-02", raw, loc)
	if err != nil {
		return time.Time{}, false
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc), true
}

// StepTimelinessInput is the authoritative input for step-level classification.
type StepTimelinessInput struct {
	IsFuture     bool
	IsCompleted  bool
	PlannedEnd   string     // delay-adjusted YYYY-MM-DD
	CompletedAt  *time.Time // authoritative StepRuntimeState.CompletedAt
	TodayHCM     time.Time  // HCM midnight
	Location     *time.Location
}

// ResolveStepTimeliness classifies one step. Returns nil when not applicable
// (future) or when authoritative inputs are insufficient.
func ResolveStepTimeliness(in StepTimelinessInput) *TimelinessStatus {
	if in.IsFuture {
		return nil
	}
	loc := in.Location
	if loc == nil {
		loc = disclosureapp.CompanyLocation("")
	}
	plannedEnd, ok := ParseHCMBusinessDate(in.PlannedEnd, loc)
	if !ok {
		return nil
	}
	today := HCMDate(in.TodayHCM, loc)

	if in.IsCompleted {
		if in.CompletedAt == nil {
			return nil
		}
		completed := HCMDate(*in.CompletedAt, loc)
		if !completed.After(plannedEnd) {
			return statusPtr(TimelinessOnTime)
		}
		return statusPtr(TimelinessCompletedLate)
	}

	if !today.After(plannedEnd) {
		return statusPtr(TimelinessWithinDeadline)
	}
	return statusPtr(TimelinessOverdue)
}

// WorkflowTimelinessInput classifies workflow from the final step only.
type WorkflowTimelinessInput struct {
	FinalStepExists      bool
	FinalStepCompleted   bool
	FinalPlannedEnd      string // delay-adjusted YYYY-MM-DD
	FinalCompletedAt     *time.Time
	TodayHCM             time.Time
	Location             *time.Location
}

// ResolveWorkflowTimeliness uses final-step delay-adjusted planned_end and
// final-step CompletedAt. It does not read disclosure DueAt or delay flags.
func ResolveWorkflowTimeliness(in WorkflowTimelinessInput) *TimelinessStatus {
	if !in.FinalStepExists {
		return nil
	}
	loc := in.Location
	if loc == nil {
		loc = disclosureapp.CompanyLocation("")
	}
	plannedEnd, ok := ParseHCMBusinessDate(in.FinalPlannedEnd, loc)
	if !ok {
		return nil
	}
	today := HCMDate(in.TodayHCM, loc)

	if in.FinalStepCompleted {
		if in.FinalCompletedAt == nil {
			return nil
		}
		completed := HCMDate(*in.FinalCompletedAt, loc)
		if !completed.After(plannedEnd) {
			return statusPtr(TimelinessOnTime)
		}
		return statusPtr(TimelinessCompletedLate)
	}

	if !today.After(plannedEnd) {
		return statusPtr(TimelinessWithinDeadline)
	}
	return statusPtr(TimelinessOverdue)
}

func statusPtr(s TimelinessStatus) *TimelinessStatus {
	v := s
	return &v
}

func timelinessStatusString(s *TimelinessStatus) *string {
	if s == nil {
		return nil
	}
	v := string(*s)
	return &v
}
