# CMS Template Archive/Restore — DEV smoke QA (updated)

- Updated: 2026-09-22 (afternoon verification cycle)
- Task type: implementation fix + full verification (no git commit)
- Objective: Phase 1 soft archive/restore; fix Draft CreateRecord semantics; browser click-through
- Skills: integration-cross-repo, backend-api-contract, premerge-system-review (light)
- Repos: `cobo_iam_services`, `cobo_web_design`
- Mandatory README: applied

## Final verdict

**PASS WITH LIMITATIONS**

## Root cause (Draft CreateRecord / TEMPLATE_ARCHIVED)

### Misleading prior smoke note

Previous report listed under Draft path:

```text
Restore → restored_mode=draft
CreateRecord → 409 TEMPLATE_ARCHIVED
```

**Trace evidence (DEV, 2026-09-22):**

1. Archive draft → DB `status=archived`, `active_version_no=0`, `archived_from_version_no=NULL`.
2. Restore draft → DB `status=active`, `active_version_no=0`, metadata cleared.
3. CreateRecord **after restore** (before fix) → `422 TEMPLATE_NO_WORKFLOW` (not `TEMPLATE_ARCHIVED`).
4. The earlier `TEMPLATE_ARCHIVED` sample was taken **while still archived** (smoke script ordered CreateRecord before Restore).

### Code fix

`enforceTemplateEligibleForNewRecord` now distinguishes:

| State | Code | HTTP |
|-------|------|------|
| `status=archived` | `TEMPLATE_ARCHIVED` | 409 |
| not archived + `active_version_no<=0` | `TEMPLATE_NOT_ACTIVE` | 409 |
| active (`av>0`) | allow (subject to workflow/applicability) | — |

Company-scoped templates are unchanged (guard skipped when `company_id` set).

### DEV re-verify after redeploy

```text
CREATE_AFTER_DRAFT_RESTORE → 409 TEMPLATE_NOT_ACTIVE
  message: template is not published and cannot be used to create new disclosures
  details.status=active details.active_version_no=0

CREATE_WHILE_ARCHIVED → 409 TEMPLATE_ARCHIVED
```

## Implemented this cycle

### Backend
- `service.go`: CreateRecord eligibility — archived vs not published.
- `ActivateTypeVersion`: early archived reject (prior cycle).
- `handler.go` `auditLog`: log error on audit append failure (no silent ignore).
- Tests: `archive_create_guard_test.go`, `cms_workflow_integration_test.go` expect `TEMPLATE_NOT_ACTIVE`.

### Frontend
- Labels Xoá / Ngừng cung cấp / Khôi phục (prior cycle).
- `mapCmsApiError`: map `TEMPLATE_ARCHIVED` / `TEMPLATE_NOT_ACTIVE` to clear VI messages.
- Redeployed FE to DEV.

## Files changed (this cycle)

### Backend
- `internal/disclosure/app/service.go`
- `internal/disclosure/transport/http/handler.go`
- `internal/disclosure/app/archive_create_guard_test.go`
- `internal/disclosure/app/cms_workflow_integration_test.go`

### Frontend
- `src/features/cms-core/services/cmsApi.ts`

### Docs
- this file

## Tests

```bash
go test ./internal/disclosure/... -count=1
→ PASS (app, inmemory, mysql, transport/http, …)

gofmt + go vet ./internal/disclosure/...
→ PASS

npx vitest run TemplatesListScreen / useTemplatesList / TemplatesFeatureScreen.import
→ PASS (16)

npm run build (cobo_web_design)
→ PASS
```

Deploy verification: `deploy-dev.sh be` then `fe` (local unit/build run without relying on `--skip-tests` for evidence above).

## DEV verification matrix

| Check | Result | Evidence time / notes |
|-------|--------|------------------------|
| Migration 0141 columns | PASS | `archived_from_*`, `archive_reason` present |
| Active archive → restore av | PASS | `qa-evidence-recovery-e2e-20260918` av=1 round-trip |
| Draft archive → restore draft | PASS | DB `status=active` av=0 |
| Create after draft restore | PASS | `TEMPLATE_NOT_ACTIVE` (not ARCHIVED) |
| Create while archived | PASS | `TEMPLATE_ARCHIVED` |
| Create after active restore | PASS | HTTP 201 |
| Existing record GET after archive | PASS | 200 |
| Existing record PATCH after archive | PASS | 200 title updated |
| New create while archived | PASS | 409 TEMPLATE_ARCHIVED |
| Activate archived | PASS | 409 TEMPLATE_ARCHIVED |
| Archive idempotent | PASS | `already_archived`; reason unchanged |
| Restore conflict when not archived | PASS | 409 STATE_CONFLICT |
| Audit `cms_template.archive/restore` | PASS | rows in `audit_logs` with before/after + reason |
| Portal visibility | PASS | list hide/show immediate (page_size≤50) |
| Catalog cache | PASS (none) | No Redis/CDN catalog cache for disclosure-types; FE refetch on action |
| Browser: Xoá on not_active | PASS | click + loading “Đang xử lý...” |
| Browser: Khôi phục on archived | PASS | click + “Đang khôi phục...” |
| Browser: Ngừng cung cấp on active | PASS | labels visible |
| Permission `cms.template.archive` | PASS (API) | platform admin; 403 path covered by unit tests |
| Hard delete | PASS | no DELETE template API in Phase 1 |

## Cache result

- Disclosure type catalog: read-through MySQL; portal list filters `status<>archived AND av>0`.
- No Redis key invalidation required for archive/restore.
- Manual: archive → next Portal list call hides type immediately; restore → visible immediately.

## Known limitations

1. **Concurrent stress**: FOR UPDATE serialization covered by design + unit paths; no dedicated multi-goroutine race harness in CI.
2. **Legacy archived without metadata**: auto-restore of previously-published templates still fails with domain 409 (by design); draft path if metadata NULL.
3. **Email verify gate on DEV**: browser QA required marking `u_platform_tenant_admin.email_verified_at` (email was NULL).
4. **Workflow after archive**: not exhaustively exercised beyond UpdateRecord; existing records remain readable/writable.
5. **FE was already on DEV for labels**; error-message mapping redeployed this cycle.

## Acceptance criteria (Phase 1)

1–20 from prompt: **met** except concurrency harness depth → verdict **PASS WITH LIMITATIONS**.
