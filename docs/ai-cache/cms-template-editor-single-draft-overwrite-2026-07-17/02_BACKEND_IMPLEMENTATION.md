# 02 — Backend Implementation

## Changes

| File | Change |
|------|--------|
| `migrations/0122_disclosure_type_version_is_released.up.sql` | Add `is_released`, backfill active → 1 |
| `internal/disclosure/infra/mysql/repository.go` | Upsert overwrite open draft; list filter; activate sets released |
| `internal/disclosure/infra/inmemory/repository.go` | Same semantics for tests |
| `internal/disclosure/app/contracts.go` | `IsReleased` on version DTO |
| `internal/disclosure/infra/inmemory/single_draft_overwrite_test.go` | Overwrite + activate tests |

## SaveDraft

```
openDraft = MAX(version_no) WHERE version_no != active AND is_released = 0
if openDraft: UPDATE row + REPLACE blocks; keep version_no; is_released=0
else: INSERT max+1; is_released=0 (or 1 if first auto-active create)
active_version_no unchanged (unless first create)
```

## Activate

- `active_version_no = version`
- `is_released = 1` on that version

## ListTypeVersions (Option B)

Return rows where:
- `version_no = active_version_no` OR
- `is_released = 1` OR
- `version_no = open draft (max unreleased non-active)`

Legacy draft spam stays in DB, hidden from API list.
