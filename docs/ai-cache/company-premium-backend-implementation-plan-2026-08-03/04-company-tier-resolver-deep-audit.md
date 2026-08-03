# CompanyTierResolver deep audit

## Location
- Package: `github.com/cobo/cobo_iam_services/internal/subscription/entitlement`
- Type: `type CompanyTierResolver func(ctx context.Context, companyID string) string` (`checker.go`)
- Impl: `NewMySQLCompanyTierResolver(*sql.DB)` (`company_tier_mysql.go`)

## Call chain (exact)
```
CheckRuntimePremium / conflict SnapshotLoader / AdminService.companyTierLookup
→ CompanyTierResolver(companyID)
→ SQL:
   memberships (status='active') 
   JOIN user_subscription_tiers (effective_to IS NULL OR > UTC_TIMESTAMP())
→ normalizeTier (Free|Premium|Enterprise)
→ max by tierRank (Enterprise=3, Premium=2, Free=1, unknown=0)
→ string tier or ""
```

## Semantics
| Question | Answer | Label |
|----------|--------|-------|
| Stable per company? | Yes for fixed membership+user tiers | LOCKED_BY_SOURCE |
| Changes when member tier changes? | Yes | LOCKED_BY_SOURCE |
| Used for commercial purchase proof? | **No** — entitlement/dispatch/conflict only | LOCKED_BY_SOURCE |
| Empty company / no tiers? | Returns `""` (treated unknown in checker) | LOCKED_BY_SOURCE |
| Errors? | Query error → `""` (silent fail-closed for entitlement) | LOCKED_BY_SOURCE |
| Cache? | **None** in resolver itself | LOCKED_BY_SOURCE |

## Verdict label
`COMPANY_TIER_RESOLVER_NOT_BILLING_SOT`

**Must not** drive Premium badge as “company purchased Premium” unless Product explicitly redefines product copy to mean “effective entitlement tier of members”.
