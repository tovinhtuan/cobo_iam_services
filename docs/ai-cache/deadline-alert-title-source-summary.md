# Deadline alert title — source & fix (2026-05-25)

## Problem

Tab **Cảnh báo thời hạn** showed redundant/wrong titles because `disclosure_records.title` was set incorrectly at record creation.

## Expected titles

| Source | Title |
|--------|--------|
| Ad-hoc (approved proposal) | First line of `change_note` (= FE proposal title) |
| Periodic (worker) | `{template active version name} — {cycle_label}` |

## Fixes

1. **Ad-hoc `AdminApprove`** — `ResolveAdHocRecordTitle(change_note, type display name, type_id)`; no `Ad-hoc:` prefix; no full change_note body.
2. **Periodic `autoRecordTitle`** — uses `PeriodicCycleRow.TypeName` from `ListPendingCycles` JOIN `disclosure_type_versions`.
3. **`GET deadline-alerts`** — `DisplayAlertTitle()` enriches legacy rows: ad-hoc line from proposal join; periodic rebuild from `type_name` + legacy `[Tự động] … — cycle` pattern.
4. **FE detail** — prefer `alert.title` (API-resolved) over `record.title`.

## Key files

- `internal/adhoc/app/record_title.go`, `service.go`
- `internal/disclosure/app/periodic.go`, `infra/mysql/repository.go` (`ListPendingCycles`)
- `internal/deadlinealerts/app/alert_title.go`, `service.go`, `infra/mysql/repository.go`

## Note

Existing DB rows keep old `disclosure_records.title` until re-created; list API still shows corrected titles via enrichment.
