# Dev account: platform CMS + tenant admin (2 companies)

**Created:** 2026-05-24  
**Migration:** `0063_dev_platform_tenant_dual_admin`

## Credentials

| Field | Value |
|-------|-------|
| Email | `platform.tenant.admin@example.com` |
| Password | `secret` |
| Tier | Enterprise |

## Memberships

| Company | ID | Roles |
|---------|-----|-------|
| Company X | `c_001` / `m_107` | `cms_operator` + `admin_doanh_nghiep` |
| Company Y | `c_002` / `m_108` | `cms_operator` + `admin_doanh_nghiep` (c_002 roles cloned) |

## Capabilities

- **Platform:** `platform.cms.view`, CMS template read/write/activate/archive/config
- **Tenant:** full admin_doanh_nghiep set per company, ad-hoc propose + process_control, workflow override

## Login troubleshooting

| Symptom | Cause | Fix |
|---------|--------|-----|
| `INVALID_CREDENTIALS` for this email | Migration `0063` not applied on server DB | `make push-migration FILE=0063_dev_platform_tenant_dual_admin.up.sql` or `make deploy-dev-migrate` after updating `run_dev_migrations.sh` |
| `login-password-key` returns **204** | RSA PEM not configured (expected on dev) | OK — FE sends plaintext `password` |
| Other dev users work (e.g. `admin.dn@example.com`) | Only this user is in 0063 | Confirms API/DB path is fine |

**2026-05-24:** `run_dev_migrations.sh` was missing `0058`–`0063`; added so compose migrate job applies seed on fresh/restart.

## Test flow

1. Login → chọn `Company X` hoặc `Company Y`
2. Portal: ad-hoc, disclosure, admin center
3. CMS menu: `/app/platform/cms` (khi session có `platform.cms.view`)
4. Switch company để kiểm thử multi-tenant
