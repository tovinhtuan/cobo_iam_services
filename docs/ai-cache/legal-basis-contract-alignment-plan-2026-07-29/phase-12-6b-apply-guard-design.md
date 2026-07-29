# Phase 12.6B — Apply guard design (future CLI — not implemented)

## Proposed future command

```bash
go run ./cmd/legal-basis-backfill \
  --environment DEV \
  --allowlist /secure/allowlist.json \
  --snapshot /secure/snapshot.json \
  --apply \
  --confirm-token '<one-time-token>'
```

## Default behavior

- **Without `--apply`:** dry planning / validation only (exit 0 on guard PASS); **zero writes**.
- Missing flags → refuse.

## Hard guards (fail closed)

| Guard | Rule |
| --- | --- |
| Environment | `--environment` must be `DEV`; refuse `staging`/`prod`/empty |
| Database | Connected database name must be `cobo_iam`; host alias must match approved DEV map (`127.0.0.1` published or compose `mysql`) |
| Allowlist | Exactly 6 unique `record_id`; all action=`WRAP_LEGACY_FLAT` |
| Snapshot | Validates against `phase-12-6b-snapshot-schema.json`; IDs match allowlist |
| Token | One-time confirm token required with `--apply`; single use |
| Freshness | Re-inventory hashes/groups match allowlist |
| Anomalies | Refuse if Group D / malformed / overflow present |
| Count | Refuse recordCount ≠ 6 |
| Wildcard | No `--all`, no table-wide UPDATE |
| SQL allowlist on RO paths | Retain inventory interceptor for freshness queries |
| Production | Explicit refuse |

## Confirmation phrase (human)

Apply execution (separate phase) requires the user to say exactly:

> Cho phép thực thi Controlled DEV Backfill theo exact allowlist đã duyệt.

Prompting for a plan is **not** mutation permission.

## Out of scope for 12.6B-Plan

This document is design-only. Creating `cmd/legal-basis-backfill` = later phase after Approval 3+.
