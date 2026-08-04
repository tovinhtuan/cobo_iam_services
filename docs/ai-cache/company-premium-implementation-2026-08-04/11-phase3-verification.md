# Phase 3 verification

## Commands

```bash
gofmt -w <touched .go>
go test ./internal/subscription/...
go test ./internal/companyaccess/app/ -run 'Plan|CompanyPlan|GetOwnCompany|MapCompany'
go test ./internal/iam/transport/http/ -run 'Companies'
go test ./internal/companyaccess/transport/http/ -run 'OwnCompany'
go vet ./internal/subscription/companyplan/ ./internal/companyaccess/app/ ./internal/iam/transport/http/ ./internal/httpserver/
git diff --check
docker compose -f docker-compose.dev.yml build api
```

## Results (2026-08-04)

| Gate | Result |
|------|--------|
| companyplan + subscription | PASS |
| own-company plan + consistency tests | PASS |
| me/companies plan tests | PASS |
| owncompany handler | PASS |
| go vet affected | PASS |
| git diff --check | clean |
| docker compose build api | PASS (exit 0) |

Open risk (not Phase 3 gate): `MYSQL_CONCURRENCY_VALIDATION_PENDING_PHASE_5`
