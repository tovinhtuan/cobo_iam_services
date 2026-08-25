# 01 — SQL membership before / after

## BEFORE (ListRows WHERE)

```text
WHERE dr.company_id = ?
  AND LOWER(TRIM(dr.status)) <> 'draft'
  + scopeClause
ORDER BY dr.created_at DESC
```

## AFTER (ListRows WHERE)

```text
WHERE dr.company_id = ?
  AND LOWER(TRIM(dr.status)) = 'draft'
  AND dr.submitted_at IS NULL
  AND (
    NOT EXISTS (
      SELECT 1 FROM periodic_cycles pc_ir
      WHERE pc_ir.record_id = dr.record_id
    )
    OR EXISTS (
      SELECT 1 FROM periodic_cycles pc
      WHERE pc.record_id = dr.record_id
        AND COALESCE(pc.open_at, pc.cycle_start) IS NOT NULL
        AND COALESCE(pc.open_at, pc.cycle_start) <= ?  -- todayHCM YYYY-MM-DD
    )
  )
  + scopeClause
ORDER BY dr.created_at DESC
```

## Args

```text
BEFORE=[companyID, ...scopeArgs]
AFTER=[companyID, todayHCM, ...scopeArgs]
```

## Preserved

```text
ACTIVE_TEMPLATE_JOIN=unchanged
ORDER_BY=unchanged
PAGINATION=still Go-side (no LIMIT/OFFSET in ListRows)
SCOPE_SQL=unchanged
```

## Not changed

```text
DUE_RESOLUTION=still Go/service
DEADLINE_STATUS_BUCKETS=unchanged
CONFIRMATION=unchanged
GO_DRAFT_FILTER=unchanged (still skips Draft after ListRows)
```
