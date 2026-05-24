# Login INVALID_CREDENTIALS — platform.tenant.admin

**Created:** 2026-05-24  
**Skill:** incident-debugger (lightweight)

## Summary

`GET /api/v1/auth/login-password-key` → **204** is correct (no RSA; plaintext password).  
`POST /api/v1/auth/login` with `platform.tenant.admin@example.com` / `secret` → **401 INVALID_CREDENTIALS** because the user row does not exist on the dev server DB yet.

Verified on `88.216.208.0:3000`:
- `admin.dn@example.com` / `secret` → **200** (user from seed `0009`)
- `platform.tenant.admin@example.com` / `secret` → **401** (requires `0063`)

Backend accepts `email` or `login_id` (`service.Login` falls back to `req.Email`).

## Root cause

Migration `0063_dev_platform_tenant_dual_admin.up.sql` was not listed in `migrations/run_dev_migrations.sh` (list ended at `0057`), so the artifacts stack never seeded `u_platform_tenant_admin`.

## Fix (dev server)

From `cobo_iam_services` (machine with `make` + SSH):

```bash
make push-migration FILE=0063_dev_platform_tenant_dual_admin.up.sql
# or apply all pending:
make deploy-dev-migrate
```

Or restart migrate after `deploy-be` copies updated `run_dev_migrations.sh` + SQL files.

## Workaround

Use existing accounts until 0063 is applied, e.g. `admin.dn@example.com` / `secret` (single company `c_001`).

**Cached for:** Team reuse, login incidents on dev server
