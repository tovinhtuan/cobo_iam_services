# Phase 12.6A — Source discovery (updated — Docker DEV)

**Date:** 2026-07-29
**Repo:** `cobo_iam_services`
**Branch:** `recovery/lost-changes-audit-20260717-153324`

## Persistence map

| Item | Value |
| --- | --- |
| Engine | MySQL 8.0.46 (live `SELECT VERSION()`) |
| Parent table | `disclosure_types` |
| Version table | `disclosure_type_versions` PK `(type_id, version_no)` |
| Flat column | `legal_basis` TEXT NULL — **present** |
| Structured column | `legal_bases_json` JSON NULL — **present** |
| Soft-delete | **None** |
| `is_released` | **Absent on this DEV** (migration `0122` not applied); inventory approximates via `version_no == active_version_no` |

## Dataset boundary

All `disclosure_type_versions` ⨉ `disclosure_types` (global + company; all statuses; all versions).
Reconciliation key: `(type_id, version_no)` + company marker + type status.

## Connection discovery (`docker compose -f docker-compose.dev.yml config`)

Masked facts only (no password / full DSN / env dump):

| Field | Value |
| --- | --- |
| Compose file | `docker-compose.dev.yml` |
| Docker service | `mysql` |
| Container | `cobo-iam-mysql` |
| Image | `mysql:8.0` |
| Database | `cobo_iam` |
| Username (masked) | `c***` |
| Published port | `3306` → container `3306` |
| Host alias (from host) | `127.0.0.1` |
| Network | `cobo_iam_services_default` |
| API env DSN source | `MYSQL_DSN` (value redacted; host-mapped service name `mysql` → published port for host tool) |
| Credential source | Compose MySQL service application user (write-capable account) |
| `MYSQL_READONLY_DSN` | Not required for 12.6A after user confirmation |

## Policy for this phase

User confirmed Phase **12.6A only** may use Compose application credential **iff**:

- no `--apply`
- SQL allowlist interceptor (fail closed)
- explicit READ ONLY transaction (`SET SESSION TRANSACTION … READ ONLY` + `BeginTx(ReadOnly)`)
- no write probe; mutations = 0

## Tooling

- Package: `internal/disclosure/app/legal_basis_inventory`
- CLI: `cmd/legal-basis-inventory --docker-dev`

## Gate status

**PASS_READ_ONLY_DRY_RUN** (live DEV inventory completed).
