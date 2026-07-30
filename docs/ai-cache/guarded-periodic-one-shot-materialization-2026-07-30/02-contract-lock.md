# Contract lock

## Input (exact)

```json
{
  "type_id": "qa-monthly-deadline-alert-202607-1785382733",
  "company_id": "c_001",
  "period": "2026-07"
}
```

## Resolved expectation

```json
{
  "period_start": "2026-07-01",
  "period_end": "2026-07-31",
  "deadline_mode": "PERIODIC",
  "deadline_days": 23,
  "deadline_unit": "WORKING_DAYS",
  "calculated_due_date": "2026-07-31",
  "planned_action": "CREATE_CYCLE_AND_DISCLOSURE_RECORD"
}
```

## CLI

- Default: preview (mutations=0)
- Apply: `--mode=apply --apply --confirm-token <token>`
- Error scope: `MATERIALIZATION_SCOPE_NOT_ALLOWED`
- Stale: `MATERIALIZATION_STATE_CHANGED`
- Calculator mismatch: `REFUSE_CALCULATOR_MISMATCH`
