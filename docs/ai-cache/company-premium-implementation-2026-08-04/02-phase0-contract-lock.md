# Phase 0 — Locked contract (Case C)

**Status:** LOCKED for implementation starting Phase 1 (pending user confirmation to proceed)

## Source of truth

- Table: `company_subscriptions` (commercial paid plan)
- Key: `company_id`
- **Not** `user.subscriptionTier`
- **Not** `CompanyTierResolver` / max-member entitlement as billing SoT
- Entitlement resolver may continue to exist for feature gating; **must not** feed Portal Premium badge

## Wire response (additive)

### Active Premium
```json
{
  "plan": {
    "code": "PREMIUM",
    "display_name": "Premium",
    "status": "ACTIVE",
    "source": "COMPANY_SUBSCRIPTION"
  }
}
```

### No plan
```json
{
  "plan": null
}
```

Field `plan` is always present on approved parent DTOs (GetOwnCompany + `/me/companies` items). Use JSON `null`, not omit, not `{}`.

## Enums

### `plan.code` (stored / wire)
- Badge-relevant: `PREMIUM`
- Other commercial codes may exist later; unknown → fail closed (treat as no badge; prefer `plan: null` at reader boundary if not ACTIVE PREMIUM for Portal display mapping — exact storage may keep row; API reader for Portal returns null when not badge-eligible **or** returns full plan and FE fail-closes — **locked FE rule:** show badge only if `code==PREMIUM && status==ACTIVE && source==COMPANY_SUBSCRIPTION`)

### `plan.status`
`ACTIVE` | `TRIAL` | `EXPIRED` | `SUSPENDED` | `CANCELLED`

### `plan.source`
`COMPANY_SUBSCRIPTION` only for Case C paid plan responses

## Badge visibility (Portal)
Show Premium **only** when:
```
plan != null
AND plan.code == "PREMIUM"
AND plan.status == "ACTIVE"
AND plan.source == "COMPANY_SUBSCRIPTION"
```
TRIAL does **not** show Premium badge unless a future Product note unlocks it.

## API placement
1. `GET /api/v1/admin/company` → top-level `plan`
2. `GET /api/v1/me/companies` items → `plan` per company  
Shared `companyplan.Reader` + shared mapper; consistency invariant for same `company_id`.

## Authz
- GetOwnCompany: existing `company.view` + subject `company_id`
- `/me/companies`: memberships of caller only
- No IDOR via arbitrary company path in Phase 3 GetOwnCompany (subject-scoped)

## Migration strategy (Phase 1 — not executed now)
- Next migration number after `0124_company_business_sectors_multi` → propose `0125_company_subscriptions`
- `company_id` VARCHAR(64) to match existing company FKs
- Effective window: `effective_from <= now AND (expires_at IS NULL OR expires_at > now)`
- Prefer **reject overlap** of two ACTIVE windows for same company (app + unique strategy TBD in Phase 1 design review)
- Rollback: paired `.down.sql`; DEV-only apply in Phase 5

## Cache
- Phase 2: **no new cache** unless separately approved (match current GetOwnCompany / resolver pattern)

## FE timing
- Phase 6+ only after Phase 5 Backend DEV API smoke PASS + user confirmation
- Remove personal `user.subscriptionTier` badge only then
- Fail closed; never fallback to user tier

## Rollback readiness (documented)
- Additive API → unwire reader / return null
- Migration down if unused in prod
- FE old ignores `plan`; FE new hides badge when null
