# Ad-hoc approval -> deadline alert confirm flow (2026-05-26)

## Mục tiêu đã chốt

- `approve` ad-hoc proposal **không** được làm alert thành `DONE`.
- Trạng thái mới `PENDING_CONFIRM` dùng cho record terminal (`Published/Completed`) nhưng chưa có xác nhận kết thúc.
- `DONE` chỉ khi gọi API confirm bằng quyền `deadline.manage`.

## Thay đổi BE đã triển khai

1. **Contract deadline alerts**
   - `internal/deadlinealerts/app/contracts.go`
   - Bổ sung status `PENDING_CONFIRM`.
   - Thêm API service `ConfirmDeadlineAlert(...)`.

2. **Status resolver**
   - `internal/deadlinealerts/app/service.go`
   - `resolveDueDateAndStatus`:
     - có bản ghi xác nhận -> `DONE`
     - terminal nhưng chưa xác nhận -> `PENDING_CONFIRM`
     - còn lại -> `UPCOMING/DUE_SOON/OVERDUE`

3. **Filter status**
   - `internal/deadlinealerts/app/status.go`
   - `normalizeStatusFilter` hỗ trợ `PENDING_CONFIRM`.

4. **Persistence confirmation**
   - Migration mới:
     - `migrations/0076_deadline_alert_confirmations.up.sql`
     - `migrations/0076_deadline_alert_confirmations.down.sql`
   - Bảng mới: `deadline_alert_confirmations`.
   - `internal/deadlinealerts/infra/mysql/repository.go`:
     - join `deadline_alert_confirmations` trong `ListRows`
     - thêm `HasDisclosureRecord`
     - thêm `ConfirmDeadlineAlert` (idempotency key support)

5. **HTTP endpoint confirm**
   - `internal/deadlinealerts/transport/http/handler.go`
   - Route mới: `POST /api/v1/company/deadline-alerts/{id}/confirm`

6. **Authorization mapping**
   - `internal/authorization/infra/mysql/repository.go`
   - action `deadline.confirm` -> required permission `deadline.manage`.

## Thay đổi FE đã triển khai

1. **Status mới**
   - `cobo_web_design/src/types.ts`:
   - `DeadlineStatus` thêm `Pending Confirm`.

2. **API client**
   - `src/services/deadlineAlertsApi.ts`
   - map `PENDING_CONFIRM <-> Pending Confirm`.
   - thêm method `confirm(recordId, { note, idempotencyKey })`.

3. **UI list/detail**
   - `src/pages/portal/DeadlineList.tsx`: badge/icon/label cho `Pending Confirm`.
   - `src/pages/portal/deadlines/DeadlineListFilterBar.tsx`: thêm filter option.
   - `src/pages/portal/DeadlineDetail.tsx` + `.../DeadlineAlertDetailSidebar.tsx`:
     - khi `Pending Confirm` dùng action confirm done thay vì publish modal.
     - kiểm tra quyền `deadline.manage`.

4. **Publish readiness**
   - `src/services/deadlinePublishReadiness.ts`
   - chặn mở publish modal khi alert ở `Pending Confirm`.

## Test đã chạy

- BE: `go test ./internal/deadlinealerts/...` -> pass.
- FE: `vitest` cho:
  - `src/services/deadlineAlertsApi.test.ts`
  - `src/services/deadlinePublishReadiness.test.ts`
  - `src/pages/portal/deadlines/DeadlineListFilterBar.test.tsx`
  -> pass.

## Ghi chú rollout

- Cần chạy migration `0076` trước khi bật endpoint confirm trên môi trường dùng MySQL.
- Tương thích ngược:
  - Nếu chưa có bản ghi trong `deadline_alert_confirmations`, alert terminal sẽ hiện `PENDING_CONFIRM` thay vì `DONE`.
