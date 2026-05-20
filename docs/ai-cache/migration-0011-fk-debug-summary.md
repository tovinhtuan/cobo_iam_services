# Migration 0011 FK Debug Summary

- task type: implement
- objective/question: fix artifact-stack migration failure where `0011_user_subscription_tiers.up.sql` violates `fk_user_subscription_tier_user`
- created: 2026-05-20
- updated: 2026-05-20
- created by: debugging-and-error-recovery

## Summary

`run_dev_migrations.sh` applies `0011_user_subscription_tiers.up.sql` before `0009_seed_authz_test_accounts.up.sql`.
Migration `0011` previously inserted deterministic tier overrides for seeded accounts such as `u_admin_web` and `u_admin_dn` before those users existed in `users`, causing MySQL error 1452 on a fresh database.
After fixing `0011`, a second fresh-DB issue surfaced: `0009_seed_authz_test_accounts.up.sql` inserted roles/memberships for company `c_001` and department membership for `d_legal` without creating those parent rows first.

## Implemented

- changed `migrations/0011_user_subscription_tiers.up.sql`
- override rows now use `INSERT ... SELECT` from an inline seed set joined with `users`
- result: override rows are only inserted when the referenced `user_id` already exists, so the migration becomes safe on a fresh DB and remains idempotent
- changed `migrations/0009_seed_authz_test_accounts.up.sql`
- seed now creates baseline parent rows it depends on:
  - `companies.c_001`
  - `departments.d_legal`
- seed now also creates the full permission set `10000000-...-0001` through `000d` before inserting `role_permissions`
- result: `0009` no longer relies on `seed_dev_identity_authorization.sql` to satisfy foreign keys on a fresh DB
- changed `migrations/0032_customize_workflow_extension.up.sql`
- aligned FK columns in `0032` at column level instead of forcing one table-wide collation
- `workflow_step_milestones.workflow_instance_id` now explicitly uses `utf8mb4_unicode_ci` to match `workflow_instances.workflow_instance_id` from `0004`
- `ad_hoc_proposals.type_id` now explicitly uses `utf8mb4_0900_ai_ci` to match `disclosure_types.type_id` from `0012` on MySQL 8 default DB collation
- result: foreign keys `fk_wsm_instance` and `fk_adhoc_type` no longer fail on fresh MySQL 8 databases with default `utf8mb4_0900_ai_ci`
- changed `migrations/0033_smoke_workflow_dev_seed.up.sql`
- replaced nonexistent smoke seed `type_id = 'dt-001'` with existing catalog type `dt-custom-obligation`
- result: smoke disclosure record and ad-hoc proposal now reference a valid `disclosure_types.type_id`

## Affected Files

- `migrations/0011_user_subscription_tiers.up.sql`
- `migrations/0009_seed_authz_test_accounts.up.sql`
- `migrations/run_dev_migrations.sh`
- `migrations/0032_customize_workflow_extension.up.sql`
- `migrations/0033_smoke_workflow_dev_seed.up.sql`

## Contracts / Constraints / Decisions

- keep `0011` before `0009` so the table still exists before later seed scripts write into `user_subscription_tiers`
- preserve default backfill of `Free` for existing users
- preserve deterministic overrides for privileged seed accounts once those users exist
- avoid disabling foreign keys or relying on manual rerun order

## Verification

- BLOCKED: local required verify `docker compose -f docker-compose.dev.yml build api`
- reason: Docker daemon not running on the current machine (`//./pipe/docker_engine` not found)

## Remaining Risks / Next Steps

- fresh DB should be re-tested by rerunning artifact-mode stack migration
- if future migrations need deterministic data overrides, prefer `JOIN users` or move the dependent seed later in the sequence explicitly
