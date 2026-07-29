# Phase 12.6B — Risk register

| Risk | L | I | Detection | Prevention | Rollback | Owner |
| --- | --- | --- | --- | --- | --- | --- |
| Stale dry-run | M | H | Freshness hash/group recheck | STOP `STALE_DRY_RUN` | n/a pre-write | Eng |
| Concurrent CMS write | M | H | CAS RowsAffected=0 | Short window + CAS | Txn rollback | Eng |
| Wrong environment | L | Crit | Env/DB name guards | DEV-only hard refuse | n/a | Eng |
| Wrong dataset boundary | L | H | Allowlist lock ALL_6 | Owner Approval 1 | Snapshot restore | PO/Eng |
| Snapshot incomplete | M | Crit | Schema validate | Abort apply | n/a | Eng |
| Partial update | M | Crit | Single txn all-6 | All-or-nothing | Txn rollback | Eng |
| UUID collision | L | H | Uniqueness assert | UUIDv7 + check | Rollback | Eng |
| Projection mismatch | M | H | In-txn read-back vs OD-7 | Use `ProjectLegalBasesToLegacy` | Rollback | Eng |
| updated_at/audit semantics | L | M | No `updated_at` column; `updated_by` system actor | Locked audit design | Restore `updated_by` | Eng |
| Migration 0122 / schema change | M | H | Schema probe | STOP if unexpected columns change dataset | Re-plan | Eng |
| Credential leakage | M | Crit | Secret scan evidence | Mask paths; no DSN in git | Rotate secrets | Eng |
| Rollback overwrite newer data | M | H | CAS on post hashes | Refuse `STALE_ROLLBACK` | Manual BA | Eng |
| Docker/deploy scope creep | M | M | Diff audit | Forbidden in apply/plan verify | Document exception | RSO |

L=likelihood I=impact (qualitative).
