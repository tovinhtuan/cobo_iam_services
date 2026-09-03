# Environment & source audit

## Environment gate

| Field | Value |
|-------|-------|
| ENVIRONMENT | **DEV** |
| PRODUCTION | false |
| DEV_HOST | `88.216.208.0:21239` (SSH) |
| DEV_DB | `cobo_iam` (MySQL in `cobo-iam-mysql` container) |
| DEV_API | `http://88.216.208.0:8080` |
| DEV_PORTAL | `http://88.216.208.0:3000` |
| Runtime | Docker Compose at `/root/cobo_project` |

Evidence: `deploy-dev.local.env.example`, live SSH queries 2026-09-01.

## Template root authority

| Entity | Actual table |
|--------|----------------|
| TEMPLATE_ROOT_TABLE | `disclosure_types` (`type_id` PK) |
| TEMPLATE_VERSION_TABLE | `disclosure_type_versions` (`type_id`, `version_no` PK) |
| Display name | `disclosure_type_versions.name` on **active** version (`disclosure_types.active_version_no`) |

Global templates: `disclosure_types.company_id IS NULL`.

## Canonical delete semantics (source)

Searched: `DeleteType`, `DeleteTemplate`, `archive`, CMS handlers.

| Finding | Detail |
|---------|--------|
| Hard delete API | **Not found** |
| Archive API | `POST /api/v1/platform/cms/templates/{type_id}/archive` |
| Archive semantics | `status='archived'`, `active_version_no=0` — hides from Portal, **row remains** |
| Workflow delete | `DELETE .../workflow` — workflow only, not template root |

Source: `internal/disclosure/infra/mysql/cms_repository.go` `ArchiveGlobalTemplate`, `internal/disclosure/transport/http/handler.go`.

```text
CANONICAL_TEMPLATE_DELETE_SUPPORTED=false
CANONICAL_DELETE_ENTRYPOINT=POST /api/v1/platform/cms/templates/{type_id}/archive (soft archive only)
DELETE_SEMANTICS=ARCHIVE_ONLY (API); user requested hard delete → requires scoped transactional SQL
```

## Worker

```text
PERIODIC_WORKER_RUNNING=true (cobo-iam-worker Up)
PERIODIC_SEEDING_ENABLED=true
DELETE_WORKER_RACE_RISK=MEDIUM
```

Worker may seed new cycles/records for active templates during cleanup window.

## Backup / rollback

```text
DEV_BACKUP_AVAILABLE=unknown
ROLLBACK_METHOD=Manual MySQL restore / re-seed; recommend mysqldump snapshot immediately before execution
```

## Application source

```text
APPLICATION_SOURCE_CHANGED=false (audit cycle)
PREEXISTING_USER_CHANGES_PRESERVED=true
```
