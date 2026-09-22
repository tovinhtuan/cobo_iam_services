# CMS Template Archive/Restore — DEV deploy + smoke QA (2026-09-22)

- Task type: implementation + DEV deploy + smoke (no git commit)
- Objective: Phase 1 soft archive/restore on DEV
- Skills: integration-cross-repo, backend-db-migration-safe, premerge-system-review (light)
- Repos: `cobo_iam_services`, `cobo_web_design`
- Mandatory README: applied

## Deployed (no commit)

| Step | Result |
|------|--------|
| Migration `0141_disclosure_type_archive_metadata` | Applied on DEV (`executed_at` 2026-09-22) |
| `deploy-dev.sh all --skip-tests` | PASS (BE + FE + migrate) |
| BE hotfix Activate early archived guard | Redeploy `be --skip-tests` PASS |
| `/healthz` `/readyz` | 200 |
| FE `:3000` | 200; bundle contains `Ngừng cung cấp`, `Khôi phục`, `/restore` |

## Smoke QA (DEV API)

Account: `platform.tenant.admin@example.com` → company `c_001` / `m_107`.

### Draft path
- Archive `qa-import-human-authorable-*` → `status=archived`, `already_archived` idempotent
- Restore → `restored_mode=draft`, `active_version_no=0`
- CreateRecord → `409 TEMPLATE_ARCHIVED`
- Restore conflict → `409 STATE_CONFLICT`

### Active path (`qa-evidence-recovery-e2e-20260918` av=1)
- Archive → `archived_from_version_no=1`, `active_version_no=0`
- Activate while archived → `409 TEMPLATE_ARCHIVED` (restore required)
- Portal list (`GET /api/v1/disclosure-types?page=1&page_size=50`): visible → hidden → visible after restore
- CreateRecord → `409 TEMPLATE_ARCHIVED`
- Restore → `restored_mode=active`, `active_version_no=1`

### Schema
Columns present: `archived_from_version_no`, `archived_at`, `archived_by`, `archive_reason`.

## Not in this deploy
- Hard delete
- Git commit / push
- Browser UI click-through (API + FE bundle markers only)

## Remaining
- Manual CMS UI click: labels Xoá / Ngừng cung cấp / Khôi phục + confirm copy
- Legacy archived rows without metadata still cannot auto-restore
