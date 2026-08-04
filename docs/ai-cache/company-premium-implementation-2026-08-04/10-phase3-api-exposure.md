# Phase 3 — Additive company plan API exposure

## Error policy (locked before implement)

**STRICT** for both endpoints (same policy):

| Behavior | Decision |
|----------|----------|
| Reader/DB error → `plan: null` | **Forbidden** |
| Reader/DB error | Propagate as HTTP **500** `INTERNAL_ERROR` (`MapCompanyPlanReadError`) |
| No covering record | `"plan": null` (success) |
| Badge filter on BE | **No** — return real status |

Rationale: silent null hides outages and breaks GetOwnCompany ↔ `/me/companies` consistency. Roles/titles on `/me/companies` historically soft-fail; plan enrichment intentionally does **not** copy that pattern.

## Wire contract

```json
"plan": { "code", "display_name", "status", "source" } | null
```

- Field always present on GetOwnCompany + `/me/companies` items
- Shared `companyplan.PlanDTO` + `ToPlanDTO` + `companyplan.Service`
- Same resolve `at` (injectable clock for tests)

## Layers

| Layer | Change |
|-------|--------|
| `companyplan/wire.go` | Shared DTO/mapper |
| `companyaccess` GetOwnCompany / PatchOwnCompany | Attach via Reader |
| `me_handler.companies` | Batch `GetEffectivePlans` on membership IDs only |
| `httpserver` | Wire shared Service (MySQL or memory) |
| CMS GetPlatformCompany | **No** reader call (Plan stays null) |

## Authz unchanged

- GetOwnCompany: `company.view` + subject `company_id`
- `/me/companies`: caller memberships only

## Explicitly not done

- DEV migrate/seed/deploy/FE
- Personal Premium removal
- Admin write API / CMS plan enrichment
- CompanyTierResolver / Enterprise badge
- Concurrency MySQL proof (`MYSQL_CONCURRENCY_VALIDATION_PENDING_PHASE_5` remains open, **not** a Phase 3 pass gate)
