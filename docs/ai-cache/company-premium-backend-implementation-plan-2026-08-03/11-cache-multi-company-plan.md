# Cache / multi-company plan

## Current state
- `CompanyTierResolver`: **no cache**
- GetOwnCompany: **no cache**
- `/me/companies`: **no Redis cache** (per-request membership query)
- Effective access cache exists separately (`WithEffectiveAccessCache`) — **do not** reuse as plan cache key

## Plan (implementation)
1. **Default:** no new cache in Phase 1–2 (match existing pattern; avoid stale badges).
2. If added later: key `company_plan:{company_id}` (+ env), never `user_id`.
3. Invalidate on: company subscription write; optional on user tier change if Case B interim.
4. FE React Query/SWR keys must include `companyId`.
5. On company switch: prefer plan from new `/me/companies` item; do not keep previous company badge during loading.
