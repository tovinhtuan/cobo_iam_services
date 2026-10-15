# 05 — Kiểm chứng

## GATE R (test tái hiện fail trên HEAD `1bd56dc`)
`go test ./internal/reminder/app/ -run '...H7...'`. Cả 6 fail đúng lý do:
- `TestNotificationRulesEvaluator_NoRowAppliesDefaults` → `skip="RULE_MISSING"`.
- `TestNotificationRulesEvaluator_WorkflowOffset1_NotGatedByCompanySchedules` → `RULE_SCHEDULE_NOT_MATCHED`.
- `TestDispatchDue_FlagOn_InAppOnly_NoEmailOneInApp` → `Sent:0 Skipped:1`.
- `TestDispatchDue_FlagOn_InAppDisabled_NoInApp` → vẫn tạo 1 in-app.
- `TestDispatchDue_FlagOn_NoRow_SendsWithDefaults` → `Sent:0 Skipped:1`.
- `TestApplyRecipientPolicies_DepartmentFocalResolvesCompanyDepartment` → `[]`.

## Backend (`cobo_iam_services`)
- `go build ./...` OK.
- `go vet ./...`: rc=1, **giống hệt HEAD** (cảnh báo có sẵn ở `internal/workflowfulfillment/required_document_gate_test.go`).
- `go test ./... -count=1`: 14 test fail, đều có trong baseline HEAD (15 fail). **0 fail mới.**
  - Baseline gồm: `TestUpdateNotificationRule_TierEnforcement_FlagOffAllowsPremium`, `TestCreateSelfServiceCompany_FeatureFlagOff`, 4 `TestIntegration_*` của httpserver, và các test khác.
  - `TestSchemaParityConditionalPeriodicity` fail ở HEAD, pass ở lần này (không liên quan).
- Test mới/đổi: `notification_rules_h7_test.go` (T1, T3, T5–T10, fallback in-app, shadow, classify, parity), các test fail-closed cũ viết lại theo hợp đồng, `admin_service_rbac_read_test.go` (T11 + `disabled`), `notification_rules_consumer_test.go` (T12), `inappnotification/app/service_counted_test.go`.
- `go test -race ./internal/reminder/app/...` đạt (perf reviewer chạy).
- gofmt: không file nào từ sạch thành lệch (các file lệch sẵn từ HEAD giữ nguyên để diff gọn).
- `docker compose -f docker-compose.dev.yml build api` OK (2 lần, lần sau sau khi sửa theo review).

## Frontend (`cobo_web_design`)
- `npx tsc --noEmit`: 12 lỗi (baseline).
- `npx vitest run`: 3.147 test, 57 fail / 19 file, trùng danh sách baseline, **0 fail mới**.
- `npm run build` OK; `npm run check:mojibake` OK.
- Test mới trong `NotificationChannelPanel.test.tsx`: 3 chế độ, ghi chú mặc định, "Đã tắt", ghi chú lịch, EN.
