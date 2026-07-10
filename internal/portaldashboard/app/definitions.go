package app

const (
	AccuracyExact       = "exact"
	AccuracyEstimate    = "estimate"
	AccuracyUnavailable = "unavailable"

	SourceDeadlineAlerts = "deadline_alerts"
	SourceAdHoc          = "ad_hoc_proposals"
	SourceInApp          = "in_app_notifications"
	SourceDerived        = "derived"

	ReasonOfficialDefinitionRequired             = "official_definition_required"
	ReasonCompletionTimestampOrDefinitionMissing = "completion_timestamp_or_definition_missing"

	KpiNeedsActionNow     = "needs_action_now"
	KpiOpenOverdue        = "open_overdue"
	KpiDueNext7Days       = "due_next_7_days"
	KpiBlockedOrException = "blocked_or_exception"
	KpiPendingApproval    = "pending_approval"
	KpiOnTimeRate         = "on_time_rate"
)

func strPtr(s string) *string     { return &s }
func floatPtr(f float64) *float64 { return &f }

type domainKpi struct {
	Value    *float64
	Unit     string
	Severity string
	Source   *string
	Accuracy string
	Reason   *string
}

func unavailableKpi(unit, reason string) domainKpi {
	return domainKpi{
		Value:    nil,
		Unit:     unit,
		Severity: "unknown",
		Source:   nil,
		Accuracy: AccuracyUnavailable,
		Reason:   strPtr(reason),
	}
}

func countKpi(value float64, unit, severity, source, accuracy string) domainKpi {
	return domainKpi{
		Value:    floatPtr(value),
		Unit:     unit,
		Severity: severity,
		Source:   strPtr(source),
		Accuracy: accuracy,
		Reason:   nil,
	}
}
