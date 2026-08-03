# Domain model analysis

| Concept | Owner | Source | Purpose | Current API | Can drive Premium badge? |
|---------|-------|--------|---------|-------------|--------------------------|
| User subscription tier | User | `user_subscription_tiers` | Personal entitlement + `/me` badge today | `/api/v1/me` | **Personal only** (legacy) |
| Company entitlement (effective) | Derived | `CompanyTierResolver` | Runtime premium schedules, conflict snapshot | Internal only | **Only if Product redefines badge = entitlement** |
| Company paid plan | Company (missing) | none | Commercial package ownership | none | **Desired product SoT — missing** |
| Company feature access | Policy | entitlement Checker | Gate features (402) | error details | No (not a badge) |
| Billing status / invoice | n/a | none | Payment | CMS upgrade QR (user-tier) | No — do not expose |

## Answers (labeled)
1. Premium commercial vs entitlement? **Product says commercial; source only has entitlement.** → `REQUIRES_PRODUCT_APPROVAL`
2. User-tier inherit to company? Resolver does max-member **for entitlement**, not declared purchase inheritance. → `LOCKED_BY_SOURCE` (behavior) + `REQUIRES_PRODUCT_APPROVAL` (badge)
3. Max-member temporary policy? Appears intentional for dispatch. → `INFERRED_FROM_SOURCE`
4. Multiple active company plans? No company plan table. → N/A until schema
5–7. Trial/Expired/Suspended display? No company statuses; user rows have `effective_to` only. → `REQUIRES_PRODUCT_APPROVAL`
8. Free null vs FREE vs omit? Open. → `REQUIRES_PRODUCT_APPROVAL`
9. Unknown plan? Resolver `""` / unknown rank 0. FE must hide badge. → recommend fail-closed
10. No members? `""`. → fail-closed
11. Switch: prefer plan on `/me/companies` to avoid stale. → recommended
12. Who reads? `company.view` for own company; membership for `/me/companies`. → `LOCKED_BY_SOURCE` pattern + `REQUIRES_SECURITY_REVIEW` for platform admin
