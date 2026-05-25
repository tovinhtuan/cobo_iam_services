# Phase 2 BE — GET /company/deadline-alerts

**Ngày:** 2026-05-25

## Endpoint

`GET /api/v1/company/deadline-alerts`

- Auth: Bearer + company context; action `deadline.view` (legacy policy + permission seed)
- Query: `status`, `q`, `start_date`, `end_date`, `page`, `page_size`

## Module layout

| Layer | Path |
|-------|------|
| App | `internal/deadlinealerts/app/` — service, status, active_department |
| MySQL | `internal/deadlinealerts/infra/mysql/repository.go` |
| In-memory | `internal/deadlinealerts/infra/inmemory/repository.go` (empty list; delegates deadline context/config) |
| HTTP | `internal/deadlinealerts/transport/http/handler.go` |
| Wire | `internal/httpserver/server.go` |

## Rules implemented

- P2: SQL `status <> 'draft'` + service skip draft
- P3: `active_departments` from `current_step_code` + `snapshot_json` (max 1 dept)
- P4: `alert_id` = `record_id`
- Due date: `planned_date` → ad-hoc `final_deadline_date` → `DeadlineCalculator`
- Status: `OVERDUE` / `DUE_SOON` / `UPCOMING` / `DONE` (terminal `completed`)

## Next

Phase 3 FE — done 2026-05-25 (`cobo_web_design`): see `deadline-alerts-phase3-fe-summary.md`.
