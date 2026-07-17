# Phase 5 — CMS Media Secret Fail-safe

## Code changes
- `internal/platform/config/config.go`
  - Added validation to reject default `dev-cms-media-secret` outside local/test runtime.
  - Reject empty `CMS_MEDIA_UPLOAD_SIGNING_SECRET`.
- `internal/httpserver/server.go`
  - Added runtime guard `validateSecurityCriticalConfig(...)` so manually built config also fails closed.
- `internal/platformcms/transport/http/media_security.go`
  - Removed fallback that silently replaced empty secret with `dev-cms-media-secret`.

## Runtime/deploy action
- Added secure values on DEV server `.env`:
  - `INTERNAL_REMINDER_TOKEN` (masked)
  - `CMS_MEDIA_UPLOAD_SIGNING_SECRET` (masked)

## Result
- Public dev runtime can no longer silently run with predictable CMS media signing secret.
