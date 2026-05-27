# Batch 2 — CMS listed companies (service + HTTP)

> **Date:** 2026-05-27  
> **Scope:** BE service, platform CMS routes, auth, tests. No FE.

## Files

| Action | Path |
|--------|------|
| Create | `internal/marketreference/app/service.go` |
| Create | `internal/marketreference/app/service_test.go` |
| Create | `internal/platformcms/transport/http/listed_companies_handlers.go` |
| Create | `internal/platformcms/transport/http/listed_companies_handlers_test.go` |
| Modify | `internal/marketreference/app/types.go` (`ListedCompanyReader`, `ErrUnavailable`) |
| Modify | `internal/platform/errors/errors.go` (`NOT_FOUND`, `SERVICE_UNAVAILABLE`) |
| Modify | `internal/platformcms/transport/http/handler.go` |
| Modify | `internal/httpserver/server.go` |

## Routes

- `GET /api/v1/platform/cms/market/listed-companies`
- `GET /api/v1/platform/cms/market/listed-companies/{symbol}`

## Auth

`requireCMSAccess` → `platform.cms.view` only (no `rbac.manage`).

## 503

`NewDisabledService()` when `VNSTOCK_MARKET_ENABLED=false` or DSN empty / no pool; `PingContext` failure → `ErrUnavailable` → `SERVICE_UNAVAILABLE`.

## Envelope

List: `data.items` + `meta.total|page|limit`. Detail: `data` object with snake_case fields.

## Verify

```bash
go test ./internal/marketreference/... ./internal/platformcms/transport/http/...
go build -o /dev/null ./cmd/api ./cmd/worker
```

## Batch 3

FE can call list/detail endpoints with CMS JWT; needs `cmsApi` + list page.
