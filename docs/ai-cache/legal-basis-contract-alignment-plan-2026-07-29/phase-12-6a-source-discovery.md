# Phase 12.6A — Source discovery

**Date:** 2026-07-29  
**Repo:** `cobo_iam_services`  
**Baseline HEAD:** `9fbc337` (Phase 12.5 SHA note); lifecycle fix `0c6dcca`  
**Branch:** `recovery/lost-changes-audit-20260717-153324`  
**Dirty:** clean  

## Persistence map (from migrations + code)

| Item | Value |
| --- | --- |
| Engine | MySQL 8+ (JSON / JSON_TABLE) |
| Parent table | `disclosure_types` (`type_id` PK, `company_id` NULL=global, `status`, `active_version_no`) |
| Version table | `disclosure_type_versions` PK `(type_id, version_no)` |
| Flat column | `legal_basis` TEXT NULL |
| Structured column | `legal_bases_json` JSON NULL (migration `0019`) |
| Soft-delete | **None** on these tables |
| Archive | `disclosure_types.status = 'archived'` (row retained) |
| Released flag | `is_released` TINYINT (migration `0122`) |

## Dataset boundary (locked for 12.6A)

**Primary:** all `disclosure_type_versions` joined to `disclosure_types` (global + company; all statuses including archived; all versions).  
**No soft-delete filter** (column absent).  
**Reconciliation key:** `(type_id, version_no)` + company marker + type status.

## Connection discovery (this agent session)

| Check | Result |
| --- | --- |
| `MYSQL_READONLY_DSN` | **UNSET** |
| `LEGAL_BASIS_INVENTORY_DSN` | **UNSET** |
| `--dsn-file` candidates | missing |
| Host `127.0.0.1:3306` / `:13306` | closed |
| Compose `cobo:cobo` | write-capable — **refused by policy** |

## Tooling added (no apply mode)

- Package: `internal/disclosure/app/legal_basis_inventory` (classify/dry-run/idempotency)
- CLI: `cmd/legal-basis-inventory` — SELECT-only; requires RO DSN; grants checked

## Gate status

**BLOCKED_READ_ONLY_ACCESS** until RO DSN is provided to the agent environment.
