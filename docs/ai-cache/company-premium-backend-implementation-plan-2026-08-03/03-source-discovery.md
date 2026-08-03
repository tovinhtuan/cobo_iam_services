# Source discovery summary

## Subscription surfaces found
1. **Table** `user_subscription_tiers` (migration `0011`) — PK `user_id`, fields `subscription_tier`, `source`, `effective_from`, `effective_to`
2. **User API** `GET /api/v1/me` → `user.subscription_tier` (`internal/iam/transport/http/me_handler.go`)
3. **User load** MySQL credentials path (`internal/iam/infra/mysql/credentials.go`)
4. **CompanyTierResolver** `internal/subscription/entitlement/company_tier_mysql.go` — max member tier
5. **Entitlement Checker** `CheckRuntimePremium` uses `ResolveCompanyTier`
6. **Wiring** `cmd/api` via `internal/httpserver/server.go` (`ResolveCompanyTier` + `WithCompanyTierLookup`); `cmd/worker` reminder dispatch
7. **Conflict snapshot** `CompanySubscriptionTier` via `CompanyTierLookup` (`internal/companyaccess/conflict/snapshot_loader.go`)
8. **CMS upgrade** still reads/writes **user** tiers (`subscription_upgrade_handlers.go` `lookupUserSubscriptionTier`)
9. **No** `company_subscriptions` / billing package table in migrations

## Company profile / list
- `GET /api/v1/admin/company` → `AdminHandler.getOwnCompany` → `adminService.GetOwnCompany` → `repo.GetCompanyPlatform` → `PlatformCompanyDetail` JSON
- Authz: `company.view` scoped to `sub.CompanyID`
- `GET /api/v1/me/companies` → membership list maps; no plan
