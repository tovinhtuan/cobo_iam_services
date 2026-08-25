# 03 — QA data inventory

```text
QA_DATA_SOURCE=product API CreateRecord (admin.dn@example.com / c_001) + reversible periodic_cycles rows tagged qa-dav1-20260825
BUSINESS_REAL_RECORD_MUTATED=false
QA_DATA_CREATED=6 Draft disclosure_records + 5 periodic_cycles (second successful run)
QA_DATA_CLEANUP_REQUIRED=true (optional; tagged cycles/records left for audit)
```

Baseline before QA: DEV had **0** Draft / **0** periodic_cycles (only 5 PendingReview on c_001).

## Selected records (second run)

| Role | record_id (prefix…) | status | submitted_at | open_at | cycle_start | planned | cycle? |
|------|---------------------|--------|--------------|--------|-------------|---------|--------|
| Pre-OpenAt | …f22c | Draft | NULL | 2026-09-01 | 2026-08-01 | 2026-09-15 | HAS_PC |
| Overdue | …f2cb | Draft | NULL | 2026-08-01 | 2026-07-01 | 2026-08-20 | HAS_PC |
| Due today | …f36b | Draft | NULL | 2026-08-25 | 2026-08-01 | 2026-08-25 | HAS_PC |
| Future | …f413 | Draft | NULL | 2026-08-10 | 2026-08-01 | 2026-09-10 | HAS_PC |
| Irregular | …f4c1 | Draft | NULL | — | — | 2026-08-18 | NO_PC |
| Submit | …f577 | Draft→PendingReview | set on submit | 2026-08-01 | 2026-07-01 | 2026-08-15 | HAS_PC |

```text
TYPE_ID=bao-cao-tuan-test (active)
COMPANY_ID=c_001
TODAY_HCM=2026-08-25
TODAY_HCM_SOURCE=businessDateHCM(Asia/Ho_Chi_Minh) in repository (Phase 1)
DB_SESSION_TIMEZONE_NOT_AUTHORITY=true
```

Note: Display title in API/FE is type name (`Báo cáo tuần test`), not create payload title — identity via record_id + due_date.
