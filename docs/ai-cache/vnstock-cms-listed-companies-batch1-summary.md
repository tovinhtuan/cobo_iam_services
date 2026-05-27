# Batch 1 — CMS listed companies (config + repository)

> **Date:** 2026-05-27  
> **Scope:** `cobo_iam_services` only — no HTTP routes, no FE.

## Files

| Action | Path |
|--------|------|
| Create | `internal/marketreference/app/types.go` |
| Create | `internal/marketreference/infra/mysql/repository.go` |
| Create | `internal/marketreference/infra/mysql/repository_test.go` |
| Modify | `internal/platform/config/config.go` |
| Modify | `internal/platform/config/config_test.go` |
| Modify | `configs/config.example.env` |
| Modify | `internal/httpserver/server.go` |

## Config

- `VNSTOCK_MYSQL_DSN` → `Config.VnstockMySQLDSN` (normalized like `MYSQL_DSN`)
- `VNSTOCK_MARKET_ENABLED` → `Config.VnstockMarketEnabled` (default `false`)

## Repository

- `List` / `Count` / `GetBySymbol` on vnstock `equity_list` + `company_profiles` (`source=kbs`)
- Search: symbol prefix when `q` matches `^[A-Za-z0-9]{1,10}$`, else `company_name LIKE`; no `tax_id` / `business_code`
- Exchange filter + list display: SQL `COALESCE(e.exchange, JSON info.exchange)`
- Partial detail: equity row without profile → `has_profile=false`, nil groups; missing symbol → `app.ErrNotFound`
- Pagination max limit 100; sort `symbol` | `company_name` | `profile_updated_at`

## httpserver

- Opens read-only vnstock pool when enabled + DSN set; closes on cleanup
- `register(..., vnstockDB)` holds pool for Batch 2 handler wiring

## Verification

```bash
go test ./internal/marketreference/...
go build -o /dev/null ./cmd/api ./cmd/worker
```

## Batch 2 blockers

- Wire `marketreferencemysql.NewRepository(vnstockDB)` into `platformcms` handler
- HTTP routes + 503 when disabled/DSN empty
- Optional `marketreference/app/service.go` thin layer
