package deadlineengine

import "errors"

var (
	// ErrEventDateNotImplemented is returned when t0_policy=event_date.
	// EVENT_DATE_NOT_IMPLEMENTED — explicit guarded backlog status. No
	// fallback, no placeholder, no guessing.
	ErrEventDateNotImplemented = errors.New("deadlineengine: t0_policy=event_date not implemented")

	// ErrInvalidT0Configuration covers all malformed T0 inputs: unknown
	// t0_policy, out-of-range cycle anchors, anchor day invalid for its
	// month, or irregular templates with neither frequency_unit nor
	// t0_policy set.
	ErrInvalidT0Configuration = errors.New("deadlineengine: invalid T0 configuration")

	// ErrInvalidDeadlineDays is returned when deadline_days <= 0 (or N <= 0
	// passed to AddDays).
	ErrInvalidDeadlineDays = errors.New("deadlineengine: deadline_days must be > 0")

	// ErrUnsupportedDayType is returned for a DayType outside {calendar, working}.
	ErrUnsupportedDayType = errors.New("deadlineengine: unsupported day type")

	// ErrStructureDeadlineUnresolved (E11): use_structure_deadline=true,
	// no matching structure entry, and deadline_days fallback is also invalid.
	ErrStructureDeadlineUnresolved = errors.New("deadlineengine: structure deadline unresolved (E11)")

	// ErrDeadlineRuleUnresolved is the orchestrator-level wrap of
	// ErrStructureDeadlineUnresolved (and other N-resolution failures).
	ErrDeadlineRuleUnresolved = errors.New("deadlineengine: deadline rule unresolved")

	// ErrRulesRequired is returned when TemplateInput.Rules is nil.
	ErrRulesRequired = errors.New("deadlineengine: applicability rules required")

	// ErrT0DateRequired: t0_policy=user_defined but RuntimeInput.T0Date is empty.
	ErrT0DateRequired = errors.New("deadlineengine: t0_date required for user_defined policy")

	// ErrInvalidT0Date: RuntimeInput.T0Date is not a valid YYYY-MM-DD date.
	ErrInvalidT0Date = errors.New("deadlineengine: invalid t0_date format")

	// ErrUnsupportedFrequency: frequency_unit set but not one of
	// {daily, weekly, monthly, quarterly, yearly} (plus day/week/month/quarter/year aliases).
	ErrUnsupportedFrequency = errors.New("deadlineengine: unsupported frequency_unit")

	// ErrWorkingDaySearchLimit: AddDays exceeded maxWorkingDayIterations
	// while searching for the next/Nth working day.
	ErrWorkingDaySearchLimit = errors.New("deadlineengine: working day search exceeded safe limit")
)
