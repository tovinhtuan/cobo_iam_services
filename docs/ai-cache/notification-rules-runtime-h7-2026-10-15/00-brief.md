# 00 — Brief: thiết lập "Kênh nhận cảnh báo" có tác dụng thật (H7 phương án c, 15/10/2026)

Workflow `wf-feature`, cỡ **thường**, hai repo: `cobo_iam_services` chính, `cobo_web_design` nhỏ.
User duyệt "làm tất cả các bước theo khuyến nghị" ngày 15/10/2026. Đồng thời user (PO) duyệt Q-23 và Q-24 trong `bug-worker-alert-fixes-2026-10-10/05-po-ba-decision-recommendations.md`.

## Mục tiêu
Admin doanh nghiệp lưu thiết lập kênh, thời điểm và người nhận cảnh báo ở Portal › Quản trị › Cài đặt › Kênh nhận cảnh báo. Thiết lập đó phải quyết định việc gửi nhắc hạn. Bật dần, không làm mất nhắc nào.

## Hiện trạng [đã xác minh, 15/10]
- Thiết lập lưu ở `notification_rules` (`company.alert_channel_prefs.v1`). DEV có 3/21 công ty đang hoạt động có dòng này (tạo 29/06–09/07).
- Runtime chỉ đọc thiết lập khi `NOTIFICATION_RULES_CONSUMER_ENABLED=true`. API và worker trên DEV đều chưa đặt biến này, nên đang tắt.
- Nếu bật nguyên trạng thì bị chặn gửi:
  - Thiếu dòng → `RULE_MISSING` (18/21 công ty mất nhắc).
  - Thiết lập "chỉ in-app" bị coi là tắt kênh.
  - Nhắc bước workflow (3 ngày, 1 ngày) không khớp lịch [30, 7, 3, 0].
  - `department_focal` dùng mã phòng ban của bước global.
- DEV, 60 ngày: 261 nhắc `SENT`, tất cả `WORKFLOW_STEP`. `reminder_configs` trống (không có nhắc hạn nộp tự cấu hình).
- API trạng thái gán cứng `dispatch_enforcement_enabled=false`. FE hiện banner "chưa dùng để gửi" (H7b).

## Thành công
1. Bật cờ không làm mất nhắc:
   - Công ty chưa lưu thiết lập dùng mặc định (Q-23).
   - Nhắc bước workflow không bị lịch công ty chặn (Q-24).
2. Kênh email và in-app được tôn trọng riêng (AHA-08). `department_focal` ánh xạ đúng phòng ban của công ty.
3. Có chế độ shadow: tính quyết định theo luật mới và ghi log/metric, không đổi việc gửi. Dùng để so sánh trước khi bật.
4. API trạng thái và FE nói đúng trạng thái: tắt / đang chạy thử / đang áp dụng.
5. Deploy DEV ở chế độ shadow. Bật enforcement khi đủ bằng chứng (xem `05-verification.md`).

## Ngoài phạm vi
- Zalo/SMS (vẫn `channel_not_implemented`).
- Escalation/quá hạn: giữ nguyên hành vi, ghi follow-up.
- Phần còn lại của PR-4/PR-5 (KPI, inactive/suspended…).

## Domain / vai trò
- CBTT: nhắc hạn liên quan nghĩa vụ công bố theo TT96/2020, xử phạt theo NĐ156/2020. Nguyên tắc: không được làm mất nhắc → fail-open với công ty chưa cấu hình.
- Admin: chỉ tenant admin (`/api/v1/admin/notification-rules/*`, quyền `admin.notification_rule.*`). Không đổi quyền, không đổi phạm vi công ty.
