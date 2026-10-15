# 01 — Thiết kế

Dựa trên RMC-07, AHA-08 và RMC-08 trong `bug-worker-alert-fixes-2026-10-10/02d-plan-pr4-pr5-list-dashboard-status.md`, thêm chế độ shadow.

## Luồng hiện tại (`internal/reminder/app`)
`DispatchDueOccurrences` → `prepareDispatch` → `evaluateDispatchDecision` (lớp 1: luật công ty, chỉ chạy khi cờ consumer bật) → lớp 2: mẫu CMS → lớp 3: resolver người nhận + lọc theo chính sách → `DispatchOccurrence` (SMTP) → in-app tạo trong goroutine nếu `allowInApp`.

## Thay đổi backend
1. **Mặc định khi thiếu dòng (Q-23)**
   - `Evaluate`: `doc == nil` → dùng `defaultAlertChannelPrefsDocument(companyID)`, giống `companyaccess.DefaultAlertChannelPrefsPayload`. Kiểm tương đương bằng parity test.
   - `AuditMetadata["defaults_applied"]=true`.
   - Reader bỏ `AND status='active'`; dòng là duy nhất theo `uk_notification_rules_company_code`. Dòng không `active` → `RULE_DISABLED`, là chủ ý của admin.
2. **Kênh riêng (AHA-08)**
   - `EvaluateDecision.ChannelAllowed{"email","in_app"}` sau các bước scope và lịch.
   - `Allowed` vẫn có nghĩa "kênh được hỏi được phép", nên simulate không đổi.
   - Runtime chỉ skip khi cả hai kênh đều không được phép. `SkipEmail = !email`, `AllowInApp = in_app`.
   - `DispatchOccurrenceRequest.SkipEmail` (`json:"-"`): không gọi SMTP, `provider_message_id="in_app_only"`, vẫn `SENT`.
   - Sau `SENT`, tạo in-app **đồng bộ** qua `InAppNotificationDeliverer` (đếm số đã tạo, `CreateForReminderCounted`). 0 bản ghi hoặc lỗi → email dự phòng cùng idempotency key (PERF-01).
   - Creator không đếm được (hoặc không có creator) thì không dùng "chỉ in-app", vẫn gửi email.
3. **Lịch chỉ áp cho nhắc hạn nộp (Q-24):** `EventType=="workflow"` thì bỏ qua so lịch, coi như khớp.
4. **`department_focal` (RMC-08):** ưu tiên bước của instance (`InstanceStepReader`), sau đó mới tới bước global, rồi `ResolveCompanyDepartment`. Không ánh xạ được thì policy này không góp người nhận.
5. **Shadow**
   - Cờ `NOTIFICATION_RULES_SHADOW_ENABLED`, chỉ có tác dụng khi consumer tắt.
   - `prepareDispatch` tính thêm một quyết định với evaluator luôn bật (`WithNotificationRulesShadow`). Truyền sẵn người nhận đã resolve để không query lại. Chỉ chạy cho dòng thật sự được gửi; dòng skip vẫn `PENDING` và được liệt kê lại mỗi tick (PERF-02).
   - So với quyết định thật rồi ghi:
     - log `notification_rules_shadow`: company, occurrence, scope, outcome, reason, kênh, số người nhận; không có email;
     - metric `reminder_rules_shadow_total{outcome,reason}`.
   - Mọi lỗi hoặc panic trong shadow đều bị chặn, không ảnh hưởng gửi thật. Panic ghi log Error kèm stack (PERF-04).
   - Outcome: `same`, `would_skip`, `would_skip_email`, `would_skip_in_app`, `recipients_changed`, `both_skip`, `shadow_error`.
6. **API trạng thái** `GET /api/v1/admin/notification-rules/status`:
   - `dispatch_enforcement_enabled = consumer`;
   - thêm `shadow_mode_enabled = shadow && !consumer`;
   - thêm `defaults_applied = (chưa có dòng)`;
   - câu cảnh báo theo trạng thái.

## Thay đổi frontend (`cobo_web_design`)
- `notificationRulesApi.ts` đọc `shadow_mode_enabled`, `defaults_applied`.
- Khối trạng thái của `NotificationChannelPanel`:
  - đang áp dụng → ghi chú xanh "đang dùng để gửi" (kèm "đang dùng mặc định" nếu chưa lưu);
  - shadow → banner "đang chạy thử, vẫn gửi theo mặc định";
  - tắt → banner hiện tại.
- Dưới "Thời điểm gửi": "Áp dụng cho nhắc hạn nộp; nhắc từng bước quy trình theo mốc của quy trình."

## Không đổi
Quyền, phạm vi công ty, schema/migration, mẫu email, Zalo/SMS, escalation.
