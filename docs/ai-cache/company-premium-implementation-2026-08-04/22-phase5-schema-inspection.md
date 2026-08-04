# Phase 5 — Schema inspection (DEV after 0125)

Inspected via `docker exec cobo-iam-mysql` on DEV (no password recorded).

## Table

`company_subscriptions` exists (InnoDB, utf8mb4_unicode_ci).

| Column | Type | Null | Notes |
|--------|------|------|-------|
| id | varchar(36) | NO | PK |
| company_id | varchar(36) | NO | FK → companies.company_id |
| plan_code | varchar(32) | NO | |
| status | varchar(32) | NO | |
| effective_from | timestamp | NO | |
| expires_at | timestamp | YES | open-ended NULL OK |
| origin | varchar(64) | NO | default `manual` |
| created_at / updated_at | timestamp | NO | defaults present |

## Indexes

- PRIMARY (`id`)
- `idx_company_subscriptions_lookup` (`company_id`,`status`,`effective_from`,`expires_at`)
- `idx_company_subscriptions_origin` (`origin`)

## FK

- `fk_company_subscriptions_company`: `company_id` → `companies.company_id`

## Verdict

**SCHEMA_INSPECTION_PASS**
