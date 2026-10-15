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

## Deploy DEV (15/10/2026) — chế độ shadow
- BE `97f5cda` (gồm `90ff8f3`), api + worker được tạo lại; binary trên server trùng sha256 với bản build local.
  - Server `.env` thêm `NOTIFICATION_RULES_SHADOW_ENABLED=true`. `NOTIFICATION_RULES_CONSUMER_ENABLED` **không đặt** (tắt).
  - Đã kiểm `printenv` trong cả api và worker.
  - Rollback: `bin/rollback/{api,worker}.pre-h7-20261015T050117Z`, `.env.bak.pre-h7-20261015T050117Z`.
- FE `0d4400b5`, bundle `index-BWoELcr9.js`, trùng sha256 bản build local. Rollback `web/releases/pre-h7-fe-20261015T061942Z.tar.gz`.
- Sau deploy:
  - `/healthz` ok, `/readyz` ready; log api/worker 0 ERROR/panic.
  - `index.html` `no-store`.
- Smoke API (`refs/api-smoke.json`; chỉ GET + simulate là POST chỉ đọc; 0 request ghi):
  - ENT / Company X (đã lưu 30/7/3/0): status `shadow=true, enforcement=false, defaults_applied=false`.
    - workflow mốc 1 → **gửi** (Q-24; trước đây `RULE_SCHEDULE_NOT_MATCHED`).
    - hạn nộp mốc 1 → `RULE_SCHEDULE_NOT_MATCHED`.
    - hạn nộp mốc 7 → gửi.
  - USER / Company Y (chưa lưu): status `defaults_applied=true, ui_state=not_configured`; mọi trường hợp → gửi (D-H7-1); trace "not gated".
- Smoke UI (`refs/ui-smoke.json`):
  - Cài đặt › Kênh nhận cảnh báo (ENT) có chip "Đang chạy thử" / "Trial run", banner chạy thử, ghi chú "Áp dụng cho nhắc hạn nộp…".
  - 0 lỗi console/HTTP.
- Hồi quy: `e2e/admin-shell-smoke.mjs` 9/9, `e2e/admin-step3-smoke.mjs` 12/12 (`refs/shell`, `refs/step3`).
- **Dữ liệu shadow:** DEV hiện có 0 nhắc chờ trong 24 giờ tới và không gửi nhắc nào trong 3 ngày qua, nên chưa có dòng `notification_rules_shadow` nào. Shadow sẽ ghi khi có nhắc thật.

## Runbook: đọc kết quả shadow và bật enforcement
1. Đọc kết quả (sau khi có nhắc được gửi, mục tiêu ≥ 1 tuần lưu lượng thật):
   - `docker logs --since 168h cobo-iam-worker 2>&1 | grep notification_rules_shadow`
     - đếm theo `outcome` / `reason`;
     - log chỉ có company_id, occurrence_id và số lượng, không có email.
   - Prometheus (DEV :9090): `sum by (outcome, reason) (increase(cobo_reminder_rules_shadow_total[7d]))`.
2. Tiêu chí bật:
   - 0 `shadow_error`;
   - 0 `would_skip` với nhắc bước workflow;
   - `would_skip` của nhắc hạn nộp chỉ ở công ty đã tự lưu lịch;
   - mọi `recipients_changed` (3 công ty đã lưu chính sách người nhận: Company X, CSKH, Nhựa An Phát Xanh) được PO xem và chấp nhận;
   - `would_skip_email` khớp các công ty chọn "chỉ in-app".
3. Bật:
   - sao lưu `.env`;
   - đặt `NOTIFICATION_RULES_CONSUMER_ENABLED=true` (shadow tự tắt khi consumer bật);
   - `docker compose -f docker-compose.artifacts.yml up -d --force-recreate --no-deps api worker`;
   - kiểm `GET /api/v1/admin/notification-rules/status` → `dispatch_enforcement_enabled=true`; FE chip "Đang áp dụng";
   - theo dõi `cobo_reminder_in_app_only_total{result="undelivered"}` = 0.
4. Rollback enforcement: đặt lại `false` rồi recreate api + worker.
   - **Trước khi rollback binary**, luôn tắt consumer trước (bản cũ chặn gửi khi thiếu thiết lập).
