# 05 — SQL correctness review

## Predicate (after `dr.company_id = ?`)

```sql
AND LOWER(TRIM(dr.status)) = 'draft'
AND dr.submitted_at IS NULL
AND (
  NOT EXISTS (periodic_cycles pc_ir WHERE pc_ir.record_id = dr.record_id)
  OR EXISTS (
    periodic_cycles pc WHERE pc.record_id = dr.record_id
      AND COALESCE(pc.open_at, pc.cycle_start) IS NOT NULL
      AND COALESCE(pc.open_at, pc.cycle_start) <= ? -- todayHCM
  )
)
```

```text
SQL_BOOLEAN_LOGIC_REVIEW=PASS
  Draft + submitted_at bind BOTH branches (parentheses correct)
EXISTS_NOT_EXISTS_REVIEW=PASS
  same relation periodic_cycles.record_id; EXISTS avoids JOIN duplication
LEGACY_FALLBACK_REVIEW=PASS
  COALESCE only inside periodic EXISTS; irregular = NOT EXISTS
HCM_BUSINESS_DATE_REVIEW=PASS
  businessDateHCM Asia/Ho_Chi_Minh bind; no CURDATE/CURRENT_DATE
NO_CREATED_AT_ALERT_FROM=PASS (created_at only ORDER BY)
NO_PLUS_7D_IN_ALERT_SQL=PASS
NO_APPLICABLE_FROM_IN_ALERT=PASS
```
