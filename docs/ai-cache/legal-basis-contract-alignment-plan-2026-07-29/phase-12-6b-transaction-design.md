# Phase 12.6B — Transaction & CAS design

**Strategy locked:** **one transaction for exactly 6 records — all-or-nothing.**

## Flow

1. Begin transaction (READ WRITE; short timeout).
2. Re-SELECT exact 6 rows by PK in allowlist stable order (type_id ASC).
3. Verify flat hash + structured empty (+ `activated_at` match snapshot).
4. Generate 6 unique UUIDs via `idgen.UUIDv7Generator`.
5. For each record: single-row `UPDATE` with CAS WHERE.
6. Assert `RowsAffected == 1` each; else rollback.
7. Read-back all 6 inside same txn; verify projection/summary/id rules.
8. Commit **only** if all PASS; else rollback.

## Compare-and-swap WHERE (pseudocode)

```sql
UPDATE disclosure_type_versions
SET
  legal_bases_json = CAST(? AS JSON),   -- marshaled DTO array
  legal_basis = ?,                      -- OD-7 projection
  updated_by = 'system:legal-basis-backfill-12.6b'
WHERE type_id = ?
  AND version_no = ?
  AND TRIM(COALESCE(legal_basis,'')) <> ''
  AND SHA2(legal_basis, 256) = ?        -- or app-side hash compare after SELECT
  AND (legal_bases_json IS NULL
       OR JSON_TYPE(legal_bases_json) <> 'ARRAY'
       OR /* no valid title/summary items — app recheck */ TRUE)
  AND activated_at = ?;                 -- from snapshot
```

**Preferred implementation:** SELECT-for-verify in txn (no `FOR UPDATE` required if race window accepted + RowsAffected CAS on exact flat text equality), then UPDATE with:

```text
WHERE type_id=? AND version_no=? AND legal_basis = ?exact_snapshot_text?
  AND legal_bases_json IS NULL
```

Affected rows:

| n | Action |
| --- | --- |
| 1 | continue |
| 0 | stale/conflict → rollback all |
| >1 | fatal → rollback all |

Forbidden: `UPDATE ... WHERE legal_basis IS NOT NULL` without PK list.

## Concurrency

- Short controlled window; no global `LOCK TABLES`.
- CMS/API concurrent save that changes flat → CAS miss → stop; do not overwrite user edit.
- No service restart required if CAS holds.

## ID generation

- 6 non-empty UUIDs, all unique within batch and vs any existing structured IDs (none today).
- Never commit generated UUID list with legal text to git; evidence may store **hashes of UUIDs** only after apply.

## Isolation note

Prefer `REPEATABLE READ` for the write txn; avoid mixing with inventory RO session.
