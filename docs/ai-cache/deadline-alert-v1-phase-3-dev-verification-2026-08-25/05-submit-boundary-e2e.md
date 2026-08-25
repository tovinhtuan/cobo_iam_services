# 05 — Submit boundary E2E

```text
SUBMIT_ACTION=POST /api/v1/disclosures/{record_id}/submit (Company API)
SUBMIT_HTTP=200
```

## Before

```text
record=…f577 status=Draft submitted_at=NULL open_at=2026-08-01 → listed in deadline-alerts
```

## After

```text
DB: PendingReview | submitted_at=2026-08-25 | cycle still linked
API: record absent from GET /company/deadline-alerts
```

```text
SUBMIT_REMOVES_DEADLINE_ALERT=PASS
INTERNAL_WORKFLOW_DELAY_IS_NOT_COMPANY_OVERDUE=PASS
Company Submit != workflow completion
```

Legacy PendingReview fixtures (pre-existing) also absent from alert list.
