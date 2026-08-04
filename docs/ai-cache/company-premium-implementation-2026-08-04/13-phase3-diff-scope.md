# Phase 3 diff-scope audit

| Path | Class |
|------|-------|
| `internal/subscription/companyplan/wire.go` (+ test) | REQUIRED |
| `internal/companyaccess/app/company_plan_enrichment.go` | REQUIRED |
| `internal/companyaccess/app/platform_company.go` | REQUIRED (`plan` field) |
| `internal/companyaccess/app/admin_service.go` / `admin_options.go` | REQUIRED |
| `internal/companyaccess/app/*plan*test.go` | REQUIRED |
| `internal/iam/transport/http/me_handler.go` (+ plan tests) | REQUIRED |
| `internal/httpserver/server.go` | REQUIRED (DI) |
| `docs/ai-cache/company-premium-implementation-2026-08-04/10–12*` | REQUIRED_DOCS |
| FE / migrations apply / deploy / CMS plan enrich | FORBIDDEN |
| CompanyTierResolver semantics | UNTOUCHED (still wired for conflict rules only) |
