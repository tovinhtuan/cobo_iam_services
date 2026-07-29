# Phase 12.6A — Read-only safety

## Status: BLOCKED_READ_ONLY_ACCESS

Agent session could **not** open a proven read-only MySQL session.

| Item | Value |
| --- | --- |
| Connection method | Not established |
| DB engine/version | Unknown (not queried) |
| Read-only role/session proof | **FAIL — DSN unset** |
| Query allowlist | SELECT + METADATA only (in CLI code) |
| Query denylist | INSERT/UPDATE/DELETE/DDL/LOCK/FOR UPDATE/--apply |
| Total statements executed | **0** |
| Write statements executed | **0** |

## Policy enforced in CLI

1. Accept only `MYSQL_READONLY_DSN` / `LEGAL_BASIS_INVENTORY_DSN` / `--dsn-file`.
2. Refuse `root:` DSN prefix.
3. Refuse if `SHOW GRANTS` implies INSERT/UPDATE/DELETE/ALL PRIVILEGES.
4. `BeginTx(ReadOnly: true)` required.
5. **No `--apply` flag exists.**

## How to unblock

1. Create MySQL user with **SELECT only** on `cobo_iam.disclosure_type_versions` + `disclosure_types` (+ `SHOW` for VERSION/GRANTS).
2. Export to agent:

```bash
export MYSQL_READONLY_DSN='ro_user:***@tcp(HOST:PORT)/cobo_iam?parseTime=true&loc=UTC&tls=false'
# or
echo 'ro_user:***@tcp(...)' > /tmp/cobo_mysql_readonly.dsn && chmod 600 /tmp/cobo_mysql_readonly.dsn
```

3. Re-run:

```bash
go run ./cmd/legal-basis-inventory --dsn-file /tmp/cobo_mysql_readonly.dsn \
  --out-dir docs/ai-cache/legal-basis-contract-alignment-plan-2026-07-29
```

Do **not** use admin/`cobo:cobo` write user “because we only SELECT”.
