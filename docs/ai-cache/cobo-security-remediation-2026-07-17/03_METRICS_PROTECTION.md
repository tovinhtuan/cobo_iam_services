# Phase 3 — Metrics Protection

## Implemented control
- `internal/httpserver/server.go`
  - Wrapped `/metrics` with `protectMetricsHandler(...)`.
  - Access policy:
    - allow loopback/private source IPs (`127.0.0.1`, RFC1918)
    - allow non-private source only when `X-Internal-Token` matches internal token
    - otherwise return `401`.

## Why this option
- Preserves internal Prometheus/container scraping.
- Blocks direct unauthenticated Internet scraping from public API port.
- Does not require immediate infra rework.

## Runtime retest
- Public request `GET /metrics` from external source: `401`.

## Tests added
- `internal/httpserver/metrics_protection_test.go`
  - public denied
  - private allowed
  - public + valid token allowed.
