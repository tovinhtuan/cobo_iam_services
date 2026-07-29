# Phase 12.6B — Snapshot runbook

**Owner:** Release Safety Owner + Backend engineer on-call
**When:** After freshness PASS, **before** any write transaction.

## Rules

1. Snapshot raw lives **outside git** (e.g. `/secure/cobo/legal-basis-backfill/<utc>/snapshot.json` or encrypted sibling).
2. `umask 077`; file mode `600`; directory `700`.
3. Optional encryption (AES-GCM) with key from env **not** logged (`LEGAL_BASIS_SNAPSHOT_KEY`) — project policy.
4. Evidence in git only records: **masked path**, **recordCount=6**, **snapshotChecksum**, allowlist ID list, timestamp.
5. Never paste `legal_basis` / JSON bodies into chat, PR, or ai-cache files.

## Capture steps (skeleton — future tool)

```text
# Pseudocode — NOT executed in Phase 12.6B-Plan
umask 077
mkdir -p /secure/cobo/legal-basis-backfill/$TS
# RO transaction SELECT exact 6 PK rows → write snapshot.json
# validate: count==6; record_ids == allowlist; each row_checksum present
chmod 600 /secure/.../snapshot.json
sha256sum snapshot.json > snapshot.sha256
```

## Validation gates

| Check | Fail action |
| --- | --- |
| recordCount ≠ 6 | ABORT |
| IDs ≠ allowlist set | ABORT |
| Missing legal_basis or row_checksum | ABORT |
| File world-readable | ABORT + rewrite perms |
| Evidence would include raw text | ABORT documentation |

## Evidence artifact (git-safe)

`phase-12-6b-snapshot-evidence.json` (created only at apply time):

```json
{
  "maskedPath": "/secure/cobo/legal-basis-backfill/<utc>/snapshot.json",
  "snapshotChecksum": "<hex>",
  "recordCount": 6,
  "recordIds": ["dt-…:1", "…"],
  "createdAt": "<utc>"
}
```
