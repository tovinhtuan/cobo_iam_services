# Portal resolved deadline display — implementation results (2026-09-22)

```text
task_type: implementation
plan: docs/ai-cache/portal-resolved-deadline-display-implementation-plan-2026-09-22.md
repos: cobo_iam_services, cobo_web_design
verdict: PASS WITH LIMITATIONS
migration: none
new_api: none
docker_compose_api_build: exit 0
```

## Implemented

### Backend (`cobo_iam_services`)

- Shared absolute-due helpers in `list_resolved_due.go`:
  - `loadPortalCycleDuesByTypeLabel`
  - `lookupPersistedCycleDueForSlot`
  - `resolveAbsoluteDueFromCycleOrPreview`
  - `attachDetailAbsoluteResolvedDue`
- List `enrichPortalListResolvedDue` refactored to use shared helpers (batch preserved).
- `DisclosureTypeDTO` additive: `resolved_due_at`, `resolved_due_source`.
- `GetTypeDetail` attaches absolute due (cycle → preview); syncs `resolved_deadline_rule.due_date` to SoT.
- Tests: cycle wins, planned_date, preview, irregular skip, list/detail parity.

### Frontend (`cobo_web_design`)

- Removed `T+N` invention; raw fallback verbatim.
- List: secondary `Hạn chót: DD/MM/YYYY` from `resolved_due_at`.
- Detail: Thời hạn / Căn cứ / Hạn chót / Nguồn + preview disclaimer.
- Absolute due authority = `resolved_due_at` (not `deadline_summary` override).
- CMS labels clarified (raw vs engine / applicability).
- Types/normalizers already mapped fields (rolling-deploy null-safe).

## Verification

| Command | Result |
|---------|--------|
| `go test ./internal/disclosure/...` | PASS |
| FE vitest (deadline + disclosure-detail + normalizers) | 137 PASS |
| `npm run build` | PASS |
| `docker compose -f docker-compose.dev.yml build api` | exit 0 |

## DEV smoke

```text
STATUS: NOT EXECUTED against live DEV in this cycle
COVERED_BY: unit/service parity tests (list/detail same cycle due; preview source; company isolation on list)
REQUIRED_FOR_FULL_PASS: manual smoke Company A (subsidiaries+cycle) vs Company B (simple/preview)
```

## Limitations

- Live multi-company browser/API smoke chưa chạy trên DEV stack.
- Full `npm test` suite toàn repo không chạy end-to-end (chạy focused packages liên quan).

## SoT locked (runtime)

```text
Semantic: resolved_deadline_rule → raw deadline_rule → chưa xác định
Absolute: CYCLE_DUE|PLANNED_DATE → DEADLINE_SUMMARY_PREVIEW → hide date
```
