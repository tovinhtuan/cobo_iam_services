# Phase 4 — PatchOwnCompany plan response safety

## Exact call order (Phase 3 bug)

```
authorize(company.edit)
→ UpdateCompanyPlatform  (COMMIT)
→ GetCompanyPlatform
→ attachOwnCompanyPlan / reader
→ if reader error → HTTP 500   ← mutation already committed
```

This violates response integrity: client sees failure after successful write.

## Remediation (option b — minimal, contract-correct)

Patch does **not** mutate `company_subscriptions`, so plan at resolve-`at` is unchanged by the PATCH body.

```
authorize(company.edit)
→ resolveOwnCompanyPlanDTO (STRICT; fail ⇒ no mutation)
→ validate sectors
→ UpdateCompanyPlatform
→ GetCompanyPlatform
→ detail.Plan = pre-resolved DTO  (no second reader call)
→ 200
```

STRICT preserved: plan errors are never remapped to `plan: null`.

## Tests

| Case | Expected |
|------|----------|
| Plan reader error before mutation | 500; company name unchanged; update count 0 |
| Update error | error status; no successful body |
| Successful update | new fields + correct plan; single plan read |
| Retry same PATCH | idempotent field write; no duplicate plan rows |
| Auth deny | no update |

Evidence tests: `admin_service_patch_plan_safety_test.go`
