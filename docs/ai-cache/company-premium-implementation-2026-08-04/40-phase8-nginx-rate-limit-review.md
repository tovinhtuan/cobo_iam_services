# Phase 8 — Nginx rate-limit review

## Exact source (`6d93e75` / `deploy-artifacts/web/nginx.conf`)

**Old**

```nginx
limit_req_zone $binary_remote_addr zone=api_per_ip:10m rate=5r/s;
# location /api/:
limit_req zone=api_per_ip burst=20 nodelay;
limit_conn conn_per_ip 20;
```

**New (loaded)**

```nginx
limit_req_zone $binary_remote_addr zone=api_per_ip:10m rate=20r/s;
# location /api/:
limit_req zone=api_per_ip burst=40 nodelay;
limit_conn conn_per_ip 40;
```

`nginx -T` on `cobo-web-design` (Phase 8): matches source — **no** `BLOCKED_NGINX_RUNTIME_CONFIG_MISMATCH`.

## Scope

- Rate limiting **still enabled** (not removed, not unlimited bypass).
- Applies to `location /api/` (and listed-lookup remains separate 10r/m).
- Only **web** recreated for zone recreate; API/worker/MySQL StartedAt unchanged; JS asset unchanged.

## Security / ops note

- Threshold raised to absorb legitimate company-switch refresh storms (~10–15 parallel `/me*` calls).
- Per-IP abuse resistance is **weaker** than 5r/s; not formally load-tested.
- Deferred: **NGINX_RATE_LIMIT_TUNING_MONITORING** (limiting-requests count, 503/429 via proxy, switch burst size, API latency/CPU, abuse by IP) — no new monitoring infra in Phase 8.

## Rollback

Restore 5r/s + burst 20 → recreate web only → known risk: switch-burst 503 returns.
