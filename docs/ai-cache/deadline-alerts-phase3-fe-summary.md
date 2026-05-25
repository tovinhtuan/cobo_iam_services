# Phase 3 FE — Wire deadline alerts & history

**Ngày:** 2026-05-25  
**Repo:** `cobo_web_design`

## API client

- `src/services/deadlineAlertsApi.ts` — `GET /api/v1/company/deadline-alerts`
- Normalizers: BE `UPCOMING|DUE_SOON|OVERDUE|DONE` → FE `DeadlineStatus`
- Query: `status`, `q`, `start_date`, `end_date`, `page`, `page_size` (+ preset time → date range)

## Pages

| File | Change |
|------|--------|
| `DeadlineList.tsx` | Tab cảnh báo: fetch API; tab lịch sử: `disclosureApi.list()` + `listTypes()` |
| `DeadlineDetail.tsx` | `id` = `record_id`; load disclosure + alert list match; `active_departments` từ API |
| `deadlineAlertViewModels.ts` | Dùng `activeDepartments` từ payload, bỏ derive template mock |

## Tests

- `deadlineAlertsApi.test.ts`
- `deadlineAlertViewModels.test.ts` (updated)
- `DeadlineListFilterBar.test.tsx` (history filter uses real today)

## Verify E2E

1. Login user có `deadline.view`
2. `/app/deadlines` — tab Cảnh báo: records sau ad-hoc approve
3. Chi tiết `/app/deadlines/:record_id` — title/due/phòng từ API
4. Tab Lịch sử — disclosures thật (không mock)
