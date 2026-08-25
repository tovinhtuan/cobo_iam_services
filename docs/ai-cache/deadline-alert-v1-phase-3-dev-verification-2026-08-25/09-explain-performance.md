# 09 — EXPLAIN / performance

Read-only MySQL `EXPLAIN` on membership-shaped query (company c_001, todayHCM bind).

| table | type | key | rows | Extra |
|-------|------|-----|------|-------|
| dr | ALL | NULL | ~17 | Using where; Using filesort |
| dt | eq_ref | PRIMARY | 1 | Using where |
| dtv | eq_ref | PRIMARY | 1 | Using index |
| pc / pc_ir EXISTS | ref | idx_pc_pending | 1 | Using where / index |

```text
DEV_EXPLAIN=FOLLOW_UP
EXPLAIN_SUMMARY=small-table full scan on disclosure_records + filesort; EXISTS uses idx_pc_pending
INDEX_CHANGE_REQUIRED_NOW=false
INDEX_FOLLOW_UP_REQUIRED=true (scale-out later; not Phase 3)
NEW_INDEX_CREATED=false
```

No pathological dependent-subquery explosion observed at DEV scale.
