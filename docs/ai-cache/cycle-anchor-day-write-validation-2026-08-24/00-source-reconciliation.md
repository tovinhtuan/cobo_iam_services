# 00 — Source reconciliation — cycle_anchor_day write validation

```text
DATE=2026-08-24
MODE=DELTA-ONLY SERVER CONTRACT HARDENING
```

## Trace

```text
CMS_CYCLE_ANCHOR_WRITE_PATH=
  PUT /api/v1/admin/disclosure-types/{type_id}/config
    → UpdateTemplateDeadlineConfig (service)
  PUT /api/v1/admin/disclosure-types/{type_id}
    → UpsertTypeVersion (service) via deadline_config.cycle_anchor_day

COMPANY_CYCLE_ANCHOR_WRITE_PATH=
  PATCH /api/v1/company/disclosure-types/{type_id}/preferences
    → UpsertCompanyTypePreference (service)

CURRENT_DAY_VALIDATOR=(none on write) — only deadlineengine.normalizeAnchor at resolve (shadow/V2)
CURRENT_BE_MIN=1 (Effective T / clamp)
CURRENT_BE_MAX=31 (Effective T / clamp)
WHY_32_IS_ACCEPTED=write paths persisted int without range check; TINYINT UNSIGNED stores 32

CANONICAL_VALIDATION_LAYER=disclosure/app.ValidateCycleAnchorDay
  (shared; called from UpdateTemplateDeadlineConfig, UpsertTypeVersion, UpsertCompanyTypePreference)
```

## Semantics preserved

```text
day=0 / omitted → allowed (unset / inherit)
ClearCycleAnchor=true → skip day validation (day payload ignored)
day 1..31 → accept
day <0 or >31 → 400 INVALID_REQUEST
NO clamp on write (32 must not become 31)
CLAMP_DAY_OF_MONTH_SOURCE_CHANGED=false
NEW_DB_MIGRATION=false
```
