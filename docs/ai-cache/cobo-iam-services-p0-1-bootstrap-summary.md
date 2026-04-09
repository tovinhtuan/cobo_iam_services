# P0.1 Bootstrap Summary

## Completed
- `go.mod` module `github.com/cobo/cobo_iam_services`, Go 1.22.
- `cmd/api`: HTTP server with graceful shutdown, `GET /healthz`, `GET /readyz`, `X-Request-Id` middleware.
- `cmd/worker`: signal-aware loop with configurable tick interval; optional MySQL ping when `MYSQL_DSN` set.
- `internal/platform`: `config`, `logger` (JSON slog), `db` (MySQL open + ping), `errors` (HTTP/API codes per `docs/api-contracts-json.md`), `httpx` (JSON + error envelope), `clock`, `idgen`.
- `configs/config.example.env` sample environment variables.

## API JSON reference
- Error JSON shape aligned with `docs/api-contracts-json.md` via `httpx.WriteError` + `platform/errors`.

## Next (P0.2)
- Add migration `0001_init_core` (IAM, company access, authz core, audit, outbox).
