# Phase 8 — Contract audit

## Commercial ownership (Case C)

- SoT: `company_subscriptions` keyed by `company_id`
- Not `user.subscriptionTier` / `user_subscription_tiers`
- Not `CompanyTierResolver` as billing SoT (entitlement resolver may exist elsewhere; not wired into Portal company `plan`)

## Backend wire

`GET /api/v1/admin/company` and `GET /api/v1/me/companies` items:

- `plan: { code, display_name, status, source } | null`
- key always present; null ≠ omit ≠ `{}`
- STRICT: reader/DB errors → HTTP 500 (Phase 4 unit authority)
- no covering row → `plan:null`
- Backend returns commercial status as stored; **does not** apply UI badge filter

### Phase 8 DEV recheck (read-only)

| Check | Result |
|-------|--------|
| me/companies c_001 Premium ACTIVE COMPANY_SUBSCRIPTION | PASS |
| me/companies c_002 plan:null | PASS |
| GetOwn c_001 ≡ me item | PASS |
| GetOwn c_002 null (platform.tenant.admin) | PASS |
| unauth GetOwn 401 | PASS |

## Frontend

| Surface | Behavior | Evidence |
|---------|----------|----------|
| Personal Ops | no `user.subscriptionTier` badge | Phase 6/7 |
| Verified email | `emailVerified===true` only | unit authority; DEV fixtures false |
| Company Information | badge iff PREMIUM+ACTIVE+COMPANY_SUBSCRIPTION | Phase 6/7 + runtime |
| Fail-closed non-ACTIVE/unknown | hidden | Phase 6 tests |
| Multi-company | no stale badge | Phase 7.1 10 rounds |

## Consistency invariant

`GetOwnCompany.plan == /me/companies[item].plan` for same `company_id` — PASS on DEV recheck.
