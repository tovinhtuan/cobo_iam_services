# Phase 12.6B — Rollback runbook

**Trigger:** Post-write verification FAIL, operator abort, or approved rollback request after successful apply with residual defect.

## Rules

1. DEV only; exact six-record allowlist; exact snapshot required.
2. All-or-nothing transaction restore of `legal_basis` + `legal_bases_json` (+ restore prior `updated_by` from snapshot).
3. CAS on **current** post-backfill state (must still match expected post hashes / UUID set from apply evidence) before restore — refuse if newer CMS edit detected (`STALE_ROLLBACK`).
4. Never `UPDATE disclosure_type_versions SET legal_bases_json=NULL` without PK allowlist.
5. Never commit raw rollback SQL containing legal text to git.

## Skeleton

```text
BEGIN;
-- for each of 6 PKs:
--   UPDATE ... SET legal_basis=snapshot.legal_basis,
--                  legal_bases_json=snapshot.legal_bases_json,
--                  updated_by=snapshot.updated_by
--   WHERE type_id=? AND version_no=?
--     AND <post-backfill predicates>;
-- assert RowsAffected==1 each;
-- read-back == snapshot checksums;
COMMIT; -- or ROLLBACK
```

## After rollback

- Re-run RO inventory: expect Groups A=6 again for allowlist keys.
- Capture `phase-12-6b-rollback-verification` evidence (hashes only).
