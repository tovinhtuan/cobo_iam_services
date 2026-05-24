# Migrate exit 1 — 0060 deadline_rule_catalog

**Created:** 2026-05-24

## Symptom

`cobo-iam-migrate` exit 1 → `api` never starts → `ERR_CONNECTION_REFUSED` on :3000.

## Root cause

`0053_cms_portal_template_tables.up.sql` creates `deadline_rule_catalog` with **PK on `code`** (no `rule_id`).

`0060_deadline_rule_catalog.up.sql` used `CREATE TABLE IF NOT EXISTS` (skipped) then `INSERT` with `rule_id`, `display_order`, … → **Unknown column 'rule_id'**.

## Fix

`0060` upgraded to detect legacy schema and `ALTER` before seed.

## Recovery (Windows)

```powershell
cd cobo_iam_services\deploy-artifacts
scp -P 21239 ..\migrations\0060_deadline_rule_catalog.up.sql root@88.216.208.0:/root/cobo_project/migrations/
scp -P 21239 ..\migrations\run_dev_migrations.sh root@88.216.208.0:/root/cobo_project/migrations/
ssh -p 21239 root@88.216.208.0 "cd /root/cobo_project && docker compose -f docker-compose.artifacts.yml up -d"
```

Or `.\push-migration.ps1 -File 0060_deadline_rule_catalog.up.sql` then `up -d`.

**Cached for:** dev server migrate failures
