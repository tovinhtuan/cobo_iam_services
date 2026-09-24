# Global CMS Record — implementation summary (2026-09-24)

```text
task type: cross-repo implement + gap close + publishing-nav removal + DEV smoke
skill: integration-cross-repo + backend-api-contract + backend-db-migration-safe + premerge-system-review
status: gaps closed; publishing CMS nav hidden; DEV smoke PASS
```

## Objective

Independent Global CMS Record (`Draft → Published → Archived`) with history-by-template, direct publish, manual materialize to Company Processing Records. Preserve legacy `/entries` company semantics.

## Publishing nav removal (2026-09-24 follow-up)

| Thành phần | Quyết định |
|---|---|
| Sidebar group “Xuất bản” | **Bỏ khỏi CMS nav** (default) |
| Review / schedule / releases routes | **Giữ** deep-link + deprecation banner |
| Backend APIs reviews/schedules/releases | **Giữ** (company-scoped semantics unchanged) |
| Legacy flag | `VITE_CMS_LEGACY_PUBLISHING_NAV=true` hiện lại nav (DEV only) |
| Global publish UX | Template History + Global Record detail |
| Smoke | `docs/ai-cache/cms-publishing-nav-removal-smoke-qa-2026-09-24/` **PASS** |

Invariant: Admin CMS publish Global CMS Record ≠ Company approve/publish Company Processing Record.

## Current → target (gap close)

| Topic | Before (phase-1) | Target (this cycle) |
|-------|------------------|---------------------|
| Migration DEV | 0142 in repo only | 0142 + 0143 applied on DEV |
| Materialized status | `Draft` (CreateRecord compat) | Storage + API = **`NotStarted`** (canonical) |
| Legacy Portal create | `Draft` | Unchanged `Draft` |
| Eligibility | `companies.status=active` only | active + entitlement ACTIVE/TRIAL + applicability + auto_create_enabled + applicable_from/to (HCM) |
| Preview reasons | active/inactive only | Stable reason codes + VI labels |
| CMS “Xuất bản” nav | Visible | **Hidden by default** |

## Status policy (final)

```text
Materialized Company Record:
  business state = NotStarted
  storage status  = NotStarted
Legacy Portal CreateRecord:
  storage status  = Draft
Submit accepts both Draft and NotStarted → PendingReview
Company counts: Draft and NotStarted both map to not_started
```

Chosen approach: **canonical storage `NotStarted`** (not Draft-compat mapping). Tests: `global_records_eligibility_test.go`, `submit_notstarted_test.go`.

## Eligibility resolver (final)

Order (fail-closed):

1. `COMPANY_INACTIVE`
2. `ENTITLEMENT_MISSING` (no ACTIVE/TRIAL subscription in window)
3. `APPLICABILITY_NOT_MATCHED` (`applicability.IsApplicable`)
4. `AUTO_CREATE_DISABLED` (`company_type_preferences.auto_create_enabled=0`; missing row = enabled)
5. `NOT_YET_APPLICABLE` / `NO_LONGER_APPLICABLE` (cycle slot vs template deadline_config; HCM)
6. `ALREADY_MATERIALIZED` / `ELIGIBLE`

Preview (`dry_run=true`) and actual materialize share `EvaluateEligibility`. Client `company_ids` re-evaluated; non-eligible skip with reason (no platform bypass).

## Migrations

| File | DEV applied | Notes |
|------|-------------|-------|
| `0142_cms_global_records.up.sql` | **yes** `2026-09-24 07:14:09` | tables + cms_record_id + perms |
| `0143_cms_global_record_id_width.up.sql` | **yes** `2026-09-24 07:30:34` | widen id/run_id/cms_record_id to VARCHAR(64) for `cms_rec_<uuid>` |

## Implemented (BE)

- `internal/platformcms/app/global_records_*.go` (service, mysql, memory, eligibility, contracts, tests)
- HTTP handlers + wire; audit actions; rbac `system_only` + enterprise deny
- Reviews list excludes Global Records
- Materialize insert sets `department_id='general'` (match CreateRecord default) + status `NotStarted`
- SubmitRecord accepts `NotStarted`

## Implemented (FE)

- `globalCmsRecordsApi.ts` + reason label VI helper; counts include approved/published/failed
- Template history tab; `/cms/records/:id`; materialize preview shows human reasons
- Permission aliases `cms.record.*`

## Smoke QA DEV

- Evidence: `docs/ai-cache/cms-global-record-smoke-qa-2026-09-24/`
- Result: **PASS** (API flows A–D core + eligibility reasons + idempotent rematerialize)
- Fixture: template `bang-tinh-luong-nhan-vien-ban-sao-2`, cycle `monthly:2026-09`, title `SMOKE-GLOBAL-CMS-2026-09-24`
- Summary highlights: eligible=4 created=4; statuses=`NotStarted`; submit→queue child=1; global_in_queue=0; rematerialize exists=4

## Verification

| Check | Result |
|-------|--------|
| `go test ./internal/platformcms/... ./internal/disclosure/app/ ./internal/authorization/...` | PASS |
| `docker compose -f docker-compose.dev.yml build api` | PASS (exit 0) |
| FE vitest globalCms + permissionGuards | PASS (8) |
| DEV deploy be + fe + migrate 0143 | PASS |
| Full `go test ./...` / full FE `npm run test` | not required this cycle (focused + docker build done) |

## Known limitations

1. No automatic fan-out; materialize is admin-triggered.
2. cycle_key frequency must align with template frequency for applicable_from (mismatch → `NOT_YET_APPLICABLE` / invalid).
3. Legacy `/cms/content/entries` still reachable by URL; nav hidden by flag only.
4. UI browser screenshots optional; this cycle evidence is API JSON pack.
5. `role_default_grant_permissions` for cms.record.* may be 0; role_permissions grants from platform.cms.view exist (12).

## Git

- no commit created
- no push performed
