---
name: cache-versioning-review
description: Dùng khi review cache và versioning cho Cobo - browser/CDN cache sai version, stale JS/CSS bundle sau deploy, chunk 404, Service Worker, header Cache-Control nginx, Redis stale cache (effective access), cache key collision, cache invalidation, versioning strategy và cache in-process. Có script kiểm tra header.
---

# Cache & Versioning Review

## When to use
- Changing nginx config, Vite build/chunking, deploy-fe, adding a service
  worker or PWA.
- Changing anything cached: Redis effective access, in-process caches,
  HTTP caching of API responses.
- Changing permissions/roles logic or the shape of cached values.
- After a deploy when users report old UI or missing chunks.

## Current state (verify; as of 2026-10)

Browser / nginx (`cobo_iam_services/deploy-artifacts/web/nginx.conf`)
- `index.html` and SPA fallback: `no-store, no-cache, must-revalidate`.
- `*.js|*.css`: `public, max-age=31536000, immutable` (any js/css, hashed or not).
- No service worker, no manifest. No CDN in front (verify per env).
- No build version exposed to the app; nothing detects "bundle older than API".

Redis (`cobo_iam_services/internal/authorization`)
- Key `cobo_iam:effective_access:{companyID}:{membershipID}`, TTL
  `EFFECTIVE_ACCESS_CACHE_TTL` (5m), fail-open to DB, in-process map if Redis
  is down at boot (per-replica, not shared).
- Invalidation only in `companyaccess/app/config_versioning.go`; other
  role/permission writes rely on TTL → stale permissions up to TTL.

In-process caches (per binary; API and worker do not share)
- Holiday year map (errors may be cached), workflow-override preview cache
  (15m, unbounded), listed-company business-code cache, opaque token map.

## Workflow

### Frontend
1. Run header check on the target environment:
   ```bash
   bash .claude/skills/cache-versioning-review/scripts/check_http_headers.sh http://<host>:3000
   ```
2. Confirm only content-hashed files get `immutable`; non-hashed files in
   `public/` (e.g. `holiday-calendar-template.xlsx`, `favicon.svg`) must not.
3. Deploy keeps previous hashed assets for at least one release (or the app
   handles chunk-load failure): long-lived tabs request old chunk names after
   deploy. In Vite, handle `vite:preloadError` with a one-time reload.
4. Missing asset must return 404, never `index.html` with 200 (the probe in
   the script shows this).
5. Version strategy: inject a build id (git SHA) via `define` and expose the
   backend version (e.g. on `/healthz`); when they diverge across a
   breaking change, prompt reload. Coordinate with
   `api-compatibility-rolling-deploy`.
6. If a service worker is ever added: versioned cache names, never cache
   `index.html` cache-first, delete old caches on activate, explicit update
   flow; document a kill switch.
7. Logout / company switch: clear company-scoped client caches and
   `sessionStorage` keys.

### Redis / in-process
1. Key design: include tenant (`companyID`) and principal; add a schema
   version segment (`...:v2:...`) whenever the cached JSON shape changes, so
   old and new binaries don't read each other's format during rollout.
2. Invalidation map: list every write that changes the cached value (role,
   permission, membership, department, delegation, break-glass, company
   status) and confirm it deletes or bumps the key; otherwise document the
   accepted staleness window.
3. Security-sensitive caches fail-closed or with a short TTL on privileged
   paths; never cache denials longer than grants.
4. Stampede: hot key expiring under load → many DB recomputations. Use
   jittered TTL and request coalescing (singleflight - new dependency needs
   approval) where load justifies it.
5. Bounded size for in-process caches; don't cache errors (or cache them very
   briefly); remember multi-replica inconsistency.
6. Tests: change permission → next request reflects it (or within the
   documented window); cache unavailable → correct fallback.

## Output format
```text
Browser cache result (headers, stale chunk behavior):
Versioning strategy:
Redis keys reviewed (format, TTL, invalidation points):
In-process caches reviewed:
Staleness windows accepted:
Findings by severity:
Tests/commands run:
```
