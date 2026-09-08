# Applicability & Deadline Config Preservation

## ApplicableFrom Preservation
- `APPLICABLE_FROM_NEXT_PERSISTENCE = PASS`
  - Normalized `NEXT_SLOT` with empty slot string persists into `disclosure_type_versions.applicable_from_mode="NEXT_SLOT"`.
- `APPLICABLE_FROM_CURRENT_PERSISTENCE = PASS`
  - Normalized `CURRENT_SLOT` persists with empty slot string.
- `APPLICABLE_FROM_SPECIFIC_PERSISTENCE = PASS`
  - `SPECIFIC_SLOT` persists with the normalized slot string (e.g. `2026-Q1`).
- Stale slot strings are never reintroduced.
- `ApplyCloneApplicableFromDefaults` is NOT invoked (preventing unwanted reset).

## ApplicableTo Preservation
- `APPLICABLE_TO_VALID_FUTURE_PERSISTENCE = PASS`
  - Valid future date (e.g. `2028-12-31`) persists as `applicable_to`.
- `APPLICABLE_TO_VALID_PAST_PERSISTENCE = PASS`
  - Valid past date (e.g. `2020-01-01`) persists as `applicable_to` in Draft v1.
  - Activation may be blocked, but Draft materialization succeeds.
- `IMPORT_VALID_NE_ACTIVATION_READY = true`
  - `ACTIVATION_READY_REQUIRED_FOR_CONFIRM = false`.
- `ApplyCloneApplicableToDefaults` is NOT invoked.

## Deadline Configuration
- Frequency Unit (`MONTHLY`, `QUARTERLY`, `YEARLY`, `DAILY`, `WEEKLY`) preserved.
- Cycle Anchors (`cycle_anchor_day`, `cycle_anchor_weekday`, `month_in_quarter`) preserved.
- Duration Type (`CALENDAR_DAYS`, `WORKING_DAYS`) preserved.
- Deadline Days preserved.
- `RUNTIME_DEADLINE_CALCULATED = false`: No dynamic schedule resolution, periodic cycle materialization, or due date generation occurs during import.
