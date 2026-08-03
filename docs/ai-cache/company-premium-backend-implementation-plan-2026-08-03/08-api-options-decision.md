# API options decision (re-evaluated)

## Option A — Extend GetOwnCompany / PlatformCompanyDetail
**Pros:** Matches Company Information screen; authz already `company.view`.  
**Cons:** Alone does not update on company switch until refetch profile; list switcher still lacks plan.

## Option B — Extend `/me/companies` items
**Pros:** Switch-time plan available; FE can key by `company_id`.  
**Cons:** N resolver queries per list (or join/batch); payload growth.

## Option C — `GET /api/v1/companies/{companyId}/plan`
**Pros:** Clear domain boundary; minimizes profile payload.  
**Cons:** Extra FE request; must duplicate authz (`company.view` + membership check); higher FE complexity for overview.

## Option D — Reuse conflict/dashboard widgets
**Rejected:** Internal ops widgets (`subscription_tier_enforced`) are not Portal company identity contract.

## Recommendation (revalidated)
**A + B with shared reader** — status: `RECOMMENDED_PENDING_APPROVAL`

### Why
- A feeds Company Information overview badge.
- B feeds multi-company switch without stale personal/user tier.
- Shared `CompanyPlanReader` prevents A/B semantic drift.

### Why not C alone
Adds hop without removing need for overview data; can be Phase-later if billing detail grows.

### Why not B alone
Company Information should not depend only on switcher cache; profile GET is source of truth for that screen.
