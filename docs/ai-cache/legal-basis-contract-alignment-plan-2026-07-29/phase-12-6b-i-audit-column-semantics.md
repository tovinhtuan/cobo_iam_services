# Phase 12.6B-I — Audit column semantics (LOCKED)

## disclosure_type_versions

| Column | Type | Null | FK | Notes |
| --- | --- | --- | --- | --- |
| updated_by | VARCHAR(64) | NOT NULL | No | DEV rows use actor `system` |
| activated_at | timestamp | NOT NULL | — | Must not change on backfill |
| created_at | timestamp | NOT NULL | — | Untouched |
| updated_at | — | — | — | Column absent |

## Decision LOCKED

Preserve `updated_by` (do not rewrite). Do not invent `system:legal-basis-backfill-12.6b`.
Do not touch `activated_at`. Rollback restores exact snapshot actor/flat/json.
Attribution of backfill lives in external apply evidence/report only.
