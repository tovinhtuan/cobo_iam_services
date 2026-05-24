# Ad-hoc: Admin DN self-assign as process controller

**Created:** 2026-05-24  
**Updated:** 2026-05-24

## Behavior

- **Create:** `admin_doanh_nghiep` may set `process_controller_membership_id` = own `membership_id` (`creatorMaySelfAssignProcessController`).
- **List:** `ListEligibleControllers` does not exclude creator when role is `admin_doanh_nghiep`.
- **Permission:** Target must have `ad_hoc_alert.process_control` (role **or** `membership_direct_permissions`).

## Bug fixed (platform.tenant.admin)

`MembershipValidator.HasPermission` / `ListMembersWithPermission` only checked `role_permissions`, not direct grants. Dev account `m_107`/`m_108` could lack `process_control` on role if migration `0062` was skipped.

**Fix:**
- `internal/adhoc/infra/mysql/membership_validator.go` — UNION direct permissions; display `login_id` when `email` empty.
- `0064_platform_tenant_admin_process_control.up.sql` — grant `process_control` to `m_107`, `m_108` + idempotent `admin_doanh_nghiep` role backfill.

## Deploy on Windows (no make)

```powershell
cd cobo_iam_services\deploy-artifacts
.\push-migration.ps1 -File 0064_platform_tenant_admin_process_control.up.sql
# Rebuild + deploy BE binary (membership_validator change)
```

**Cached for:** ad-hoc create flow, platform tenant admin testing
