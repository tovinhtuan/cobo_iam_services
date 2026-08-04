# Phase 4 diff-scope audit

| Path | Class |
|------|-------|
| `internal/companyaccess/app/admin_service.go` | REQUIRED_FIX (plan before PATCH mutation) |
| `internal/companyaccess/app/company_plan_enrichment.go` | REQUIRED_FIX |
| `internal/companyaccess/app/admin_service_patch_plan_safety_test.go` | REQUIRED_TEST |
| `internal/companyaccess/app/company_plan_consistency_matrix_test.go` | REQUIRED_TEST |
| `internal/companyaccess/transport/http/admin_handler_plan_security_test.go` | REQUIRED_TEST |
| `internal/subscription/companyplan/migration_0125_static_test.go` | REQUIRED_TEST |
| `docs/ai-cache/.../14–19*` | REQUIRED_DOCS |
| FE / DEV migrate apply / deploy / CMS enrich / Admin write-plan | FORBIDDEN |
| Unrelated generated files | NOT_COMMITTED |
