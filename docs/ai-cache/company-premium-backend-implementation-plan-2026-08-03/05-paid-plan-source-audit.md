# Paid plan source audit

## Search coverage
Migrations, `internal/subscription/**`, companyaccess, IAM me handlers, platformcms upgrade, seeds — no company-owned billing/package/license table.

## Case classification: **Case B**

> Chỉ có entitlement derived từ members (`user_subscription_tiers` + max-tier resolver).  
> Không phải paid company plan source.

### Implications
1. **Cannot** implement FE company Premium from a trustworthy billing SoT today.
2. Implementation paths after Product approval:
   - **B1 Interim (Product-approved entitlement display):** expose resolver result as `plan` with explicit `source: "member_max_entitlement"` (or omit display_name that implies purchase).
   - **B2→C Proper:** create `company_subscriptions` (or equivalent) as commercial SoT; backfill; admin/billing write path; resolver may remain for entitlement or be aligned to company row.

## Case A / C
- **Case A:** Not found.
- **Case C:** Required if Product rejects entitlement-as-badge semantics.

## Evidence seeds (user-scoped still)
- `0011_user_subscription_tiers`
- Dev overrides `0092_dev_premium_subscription_tvttthptlvh`, `0078_dev_subscription_expiry_seed`
- Registration inserts Free tier per user
