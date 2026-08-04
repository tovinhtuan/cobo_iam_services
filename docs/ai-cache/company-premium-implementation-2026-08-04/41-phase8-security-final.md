# Phase 8 — Security final

| Control | Status |
|---------|--------|
| Production touched | **false** |
| Credentials/tokens/JWT in evidence | **false** |
| Billing-sensitive fields on company plan APIs | **none observed** |
| Cross-company leakage (me/companies caller-only) | PASS (Phase 5 + Phase 8) |
| GetOwnCompany subject-scoped | PASS |
| CMS platform detail not Reader-enriched | PASS (Phase 5) |
| user.subscriptionTier fallback for company badge | **false** |
| CompanyTierResolver as billing SoT | **false** |
| Nginx rate limiting still active | **true** |
| Direct `:8080` tests did not change exposure | read-only client checks only |
| Company/membership/plan mutation in Phase 8 | **none** |
| DEV fixture `origin=dev_fixture` | present for c_001 only |

Do **not** claim `:8080` is private without network-policy evidence.
