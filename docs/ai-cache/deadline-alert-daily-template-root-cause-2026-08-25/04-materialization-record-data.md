# 04 — Materialization + disclosure_records

```text
SELECT COUNT(*) FROM disclosure_records WHERE type_id=bang-tinh-luong-nhan-vien-thang-ban-sao → 0

CHECK_10_MATERIALIZED=NOT_REACHED
RECORD_STATUS=null
RECORD_SUBMITTED_AT=null
CHECK_11_NEEDS_COMPANY_ACTION=NOT_REACHED
MATERIALIZATION_ERROR_FOUND=false
```

No cycle → MaterializePeriodicDisclosures has nothing to attach; creator never called for this template.

Materialization lookahead (`COALESCE(OpenAt,T,Due) <= now+7d`) is **not** the gating failure here — seed never ran.