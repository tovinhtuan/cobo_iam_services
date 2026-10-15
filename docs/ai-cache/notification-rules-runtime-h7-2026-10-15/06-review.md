# 06 — Review song song (Phase 6)

Ba reviewer chạy song song: `be-security-reviewer`, `api-compat-reviewer`, `perf-reliability-reviewer` (read-only, trên diff chưa commit). Không có Critical/High.

| ID | Mức | Nội dung | Xử lý |
|---|---|---|---|
| PERF-01 | Medium | Chỉ in-app: `SENT` trước khi tạo in-app; in-app tạo trong goroutine, mất im lặng khi không có user khớp, lỗi ghi, hoặc SIGTERM | Tạo in-app đồng bộ sau `SENT`, có đếm. 0 hoặc lỗi → email dự phòng (idempotency key cũ). Metric `reminder_in_app_only_total{result}`, log Warn/Error. Creator không đếm được → gửi email. Còn lại: SIGTERM đúng giữa `SENT` và tạo in-app (cửa sổ rất ngắn, cùng loại rủi ro với đường in-app hiện có) |
| PERF-02 | Medium | Shadow đếm lại dòng skip mỗi tick (dòng skip vẫn `PENDING`) | Shadow chỉ chạy khi quyết định thật là gửi. Rủi ro có sẵn (dòng skip kẹt `PENDING`, ≥50 dòng có thể chặn hàng đợi) → follow-up |
| API-01 | Medium | Rollback BE khi consumer bật → BE cũ skip | Quy tắc rollback trong `02-contract.md`: tắt consumer trước |
| API-02 | Medium | API trạng thái bỏ qua dòng không `active` | `ui_state="disabled"` + cảnh báo; FE "Đã tắt" + ghi chú. Repo in-memory trả đúng `status` (trước gán cứng `active`) |
| BES-01 / PERF-03 | Low | Dòng tắt có payload hỏng → lỗi evaluator, retry rồi FAILED | Reader kiểm `status` trước khi parse → `RULE_DISABLED` |
| PERF-04 | Low | `recover` mất thông tin, panic lần 2 thoát ra | Log Error kèm giá trị + stack, không gọi lại `emitShadow` |
| API-03 | Low | Trace simulate sai cho dòng tắt / thiếu bước mặc định | Thêm nhánh `RULE_DISABLED` và bước "system defaults applied" |
| BES-02 / API-05 | Info/Low | Hợp đồng ghi mặc định có lịch + 3 chính sách, code không | Sửa `02-contract.md`, ghi D-H7-1 |
| (parity) | Info | Simulate thiếu `InstanceStepReader` → `department_focal` khác runtime | Thêm vào `simDeps` (`httpserver/server.go`) |
| API-04 | Low | Widget dashboard vận hành (đang ẩn) hiểu cờ theo nghĩa cũ | Follow-up |
| API-05 (docs) | Low | Postman/OpenAPI/contract JSON thiếu status/simulate (có từ trước); env docs thiếu cờ `NOTIFICATION_RULES_*` | Follow-up |

Đã kiểm, không có lỗi (theo reviewer):
- Cô lập tenant ở ánh xạ phòng ban và snapshot instance.
- Chính sách chỉ thu hẹp người nhận.
- Log shadow không có email/PII; nhãn metric là hằng số.
- `SkipEmail` không thể đặt qua JSON.
- Quyền và phạm vi của API trạng thái giữ nguyên.
- Cờ tắt → hành vi y như cũ. `go test -race ./internal/reminder/app/...` đạt (perf reviewer chạy).
