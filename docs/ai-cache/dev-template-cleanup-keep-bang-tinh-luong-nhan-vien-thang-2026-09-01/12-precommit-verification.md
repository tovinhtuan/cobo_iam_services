# 12 — Pre-commit verification

All gates evaluated **inside** the open transaction before COMMIT/ROLLBACK.

| Gate | Value | Pass |
|------|-------|------|
| PRECOMMIT_TEMPLATE_ROOT_COUNT | 1 | yes |
| PRECOMMIT_KEEP_ROOT_PRESENT | true | yes |
| KEEP name exact | Bảng tính lương nhân viên tháng | yes |
| PRECOMMIT_KEEP_STATUS_UNCHANGED | active | yes |
| PRECOMMIT_KEEP_ACTIVE_VERSION_UNCHANGED | 1 | yes |
| PRECOMMIT_KEEP_VERSION_BASELINE_UNCHANGED | 1 | yes |
| PRECOMMIT_KEEP_CYCLES_UNCHANGED | 8 | yes |
| PRECOMMIT_KEEP_RECORDS_UNCHANGED | 8 | yes |
| PRECOMMIT_KEEP_TEMPLATE_BLOCKS_UNCHANGED | 6 | yes |
| PRECOMMIT_KEEP_DISPLAY_GROUPS_UNCHANGED | 2 | yes |
| PRECOMMIT_DELETE_ROOT_REMAINING | 0 | yes |
| PRECOMMIT_ORPHAN_ROWS | 0 | yes |
| PRECOMMIT_GLOBAL_MASTER_DATA_CHANGED | false (20/95/45/63) | yes |
| UNEXPLAINED_DELETE_COUNT_DELTA | 0 | yes |

```text
Decision: COMMIT
DELETE_TRANSACTION_COMMITTED=true
DELETE_ROLLED_BACK=false
```
