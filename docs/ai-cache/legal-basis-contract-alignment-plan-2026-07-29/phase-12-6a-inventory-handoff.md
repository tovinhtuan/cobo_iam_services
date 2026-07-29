# Phase 12.6A — Inventory handoff (closed)

## Dual verdict

| Axis | Verdict |
| --- | --- |
| **Operational** | **PASS_READ_ONLY_DRY_RUN** |
| **Governance** | **FAIL_SCOPE_CREEP** |

## Operational summary

- Environment: DEV (docker-compose.dev.yml MySQL)
- Connection (masked): host=`127.0.0.1` port=`3306` db=`cobo_iam` user=`c***` service=`mysql` / `cobo-iam-mysql`
- Engine: MySQL 8.0.46
- Database mutations: **0**
- Inventory validity: **VALID**
- Re-run required before apply: **NO**, unless dataset changes (freshness recheck still mandatory immediately before any future apply)

### Dataset (last successful RO inventory)

| Metric | Value |
| --- | --- |
| Total versions | 6 |
| Group A | 6 |
| B / C / D / E | 0 |
| Dry-run | WRAP_LEGACY_FLAT × 6 |
| Malformed / overflow / violations / Group D | 0 |
| Idempotency | PASS |

## Governance exception

See `phase-12-6a-scope-exception.md`.

- Exception command: `docker compose -f docker-compose.dev.yml build api`
- Database impact: none
- Inventory validity: not affected

## Phase 12.6B

- **Plan docs:** Phase 12.6B-Plan (this folder `phase-12-6b-*`)
- **Apply / mutation:** **NOT STARTED** — requires explicit user phrase + approvals

## Related evidence

- `phase-12-6a-group-summary.json`, `phase-12-6a-dry-run-preview.json`, `phase-12-6a-read-only-safety.md`, `phase-12-6a-query-log.md`
