package app

import "time"

// matchScheduleAtDispatch returns true when an enabled schedule offset matches the
// calendar-day difference between dueAt and scheduledAt (CQ-01 dispatch-time only).
func matchScheduleAtDispatch(doc *AlertChannelPrefsDocument, dueAt, scheduledAt time.Time) (bool, []ScheduleMatch) {
	if doc == nil || len(doc.Schedules) == 0 {
		return false, nil
	}
	if dueAt.IsZero() || scheduledAt.IsZero() {
		return false, nil
	}
	loc := reminderCalculatorLocation()
	dueDay := truncateToCalendarDay(dueAt, loc)
	schedDay := truncateToCalendarDay(scheduledAt, loc)
	offsetDays := int(dueDay.Sub(schedDay).Hours() / 24)

	var matched []ScheduleMatch
	for _, sp := range doc.Schedules {
		if !sp.Enabled {
			continue
		}
		kind := sp.Kind
		if kind == "escalation" {
			continue
		}
		if sp.OffsetDays == nil {
			continue
		}
		if *sp.OffsetDays != offsetDays {
			continue
		}
		matched = append(matched, ScheduleMatch{
			OffsetDays:  sp.OffsetDays,
			Kind:        kind,
			PremiumOnly: sp.PremiumOnly,
		})
	}
	return len(matched) > 0, matched
}

func truncateToCalendarDay(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}
