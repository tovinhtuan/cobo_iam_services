# 01 — Solution (IAM summary)

Full document:  
`cobo_web_design/docs/ai-cache/tenant-alerts-unified-proposals-plan-2026-09-19/01-solution.md`

## BE source highlights

| Item | Value |
|---|---|
| Alert list | `internal/deadlinealerts` — `GET /api/v1/company/deadline-alerts` — authz `deadline.view`; confirm uses `deadline.confirm` |
| Proposal list | `internal/adhoc` — `GET /api/v1/company/ad-hoc-proposals` — `ad_hoc_alert.read` or `scope=my` + propose/read; sort fixed `created_at DESC` |
| Commands | create/patch/submit/approve/reject/cancel unchanged |
| Materialize | Unanimous approve finalize → deterministic `record_id` + `workflow_instance_id` on proposal |
| Unified feed | **Does not exist** (dashboard overview ≠ merged list) |
| Deep links | Email + in-app notif → `/app/ad-hoc-proposals/...` — must keep routes |

## Phase 1 BE stance

```text
API_CONTRACT_CHANGE=false
DB_MIGRATION_REQUIRED=false
BE_CHANGE_REQUIRED=false
AUTHZ_CHANGE=false
```

Regression only: `go test ./internal/adhoc/... ./internal/deadlinealerts/...`

## Phase 2 (optional)

Additive read model `alerts-feed` with tenant isolation, permission-aware projection, namespaced status, deterministic sort/page — **separate implement task**.
