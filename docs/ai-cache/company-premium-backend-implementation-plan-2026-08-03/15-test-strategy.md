# Test strategy

## Unit (`companyplan`)
Premium/Free/empty/unknown; max of Free+Premium+Enterprise; effective_to expired excluded; no members → omit; source field set.

## Repository (Case C)
Isolation by company_id; single ACTIVE; overlap rejection; status filters.

## Service
GetOwnCompany attaches plan; auth denied without company.view; PatchOwnCompany does not accept plan writes from enterprise admin (unless Product says otherwise).

## Handler/API
200 + plan; 200 omit plan; 400 no company context; 403; me/companies plan per item; cross-user cannot see others.

## Consistency
Same company: GetOwnCompany.plan.code == me/companies item.plan.code

## Cache
If none: document. If added: key company_id; no cross-tenant.

## FE future
Personal Premium removed; company Premium shown; non-premium hidden; switch no stale; loading clears badge; unknown hidden.
