# 02 — Hợp đồng (GATE A)

**GATE A:** user duyệt ngày 15/10/2026 bằng yêu cầu "làm tất cả các bước và theo khuyến nghị". Yêu cầu này bao gồm Q-23, Q-24 và lộ trình shadow → bật. Không dừng riêng để hỏi; đã ghi lại ở đây.

## Cấu hình (env backend, api + worker)
| Biến | Mặc định | Ý nghĩa |
|---|---|---|
| `NOTIFICATION_RULES_CONSUMER_ENABLED` | false | Có sẵn. true thì thiết lập công ty **quyết định** việc gửi |
| `NOTIFICATION_RULES_SHADOW_ENABLED` | false | **Mới.** true (và consumer false) thì tính quyết định theo thiết lập, ghi log/metric, **không đổi** việc gửi |

## API — `GET /api/v1/admin/notification-rules/status` (quyền `admin.notification_rule.list`, phạm vi công ty theo token)
Thêm field, không xoá hay đổi kiểu field cũ:
| Field | Kiểu | Giá trị |
|---|---|---|
| `dispatch_enforcement_enabled` | bool | **Đổi giá trị:** trước luôn false, nay bằng `runtime_consumer_enabled` |
| `shadow_mode_enabled` | bool | **Mới.** `shadow && !consumer` |
| `defaults_applied` | bool | **Mới.** Công ty chưa có dòng thiết lập, khi áp dụng sẽ dùng mặc định |
| `ui_state` | string | Thêm giá trị `disabled`: dòng thiết lập không `active`, khi áp dụng thì không gửi nhắc (API-02) |
| `warnings` | []string | Đổi câu chữ theo trạng thái (FE hiện không hiển thị) |

`POST /api/v1/admin/notification-rules/simulate`: không đổi request; trace thêm bước (bổ sung). Vẫn dùng evaluator luôn bật, nên kết quả simulate theo luật mới.

## Hành vi gửi khi consumer bật
| Tình huống | Trước | Sau |
|---|---|---|
| Công ty chưa có dòng | skip `RULE_MISSING` | **gửi y như hôm nay** (D-H7-1): email + in-app, không chặn theo lịch, không lọc người nhận; audit `defaults_applied` |
| Dòng không `active` | coi như thiếu → skip | skip `RULE_DISABLED` (admin đã tắt) |
| Chỉ in-app | skip `CHANNEL_DISABLED` | claim → `SENT` (`in_app_only`), tạo in-app **đồng bộ** có đếm. Không tạo được hoặc lỗi → gửi email dự phòng cùng idempotency key. Metric `reminder_in_app_only_total{result=in_app,email_fallback,undelivered}`. Creator không đếm được → gửi email như thường |
| Tắt in-app | vẫn tạo in-app | chỉ email |
| Tắt cả hai | skip | skip `CHANNEL_DISABLED` |
| Nhắc bước workflow, mốc 1 ngày | skip `RULE_SCHEDULE_NOT_MATCHED` | gửi (lịch công ty không áp) |
| Nhắc hạn nộp, mốc không có trong lịch | skip | skip (không đổi) |
| `department_focal` | phòng ban của bước global | phòng ban của công ty, ánh xạ từ bước của instance/global |

Khi consumer tắt và shadow tắt: **hành vi y như hiện tại** (đã có test hồi quy).

## Lỗi / idempotency
- Không mã lỗi HTTP mới.
- Idempotency của email (`uk_email_notifications_idempotency`) và occurrence không đổi. Trường hợp `in_app_only` vẫn đi qua claim và `SENT` như cũ; in-app at-most-once như cũ.
- Shadow không ghi DB, không gửi.

## Migration
Không có.

## Tương thích khi deploy lệch phiên bản
| Kết hợp | Kết quả |
|---|---|
| BE mới + FE cũ | FE cũ đọc `runtime_consumer_enabled && dispatch_enforcement_enabled` nên hiện banner đúng; bỏ qua field mới |
| BE cũ + FE mới | FE mới không thấy `shadow_mode_enabled`/`defaults_applied` (false), hiện banner "chưa áp dụng" như cũ |
| api mới + worker cũ (hoặc ngược lại) | Chỉ khác khi bật consumer. **Bật consumer chỉ sau khi cả api và worker đã chạy bản mới** |

Thứ tự deploy: BE (api + worker, cùng lúc) → FE. Bật shadow cùng lúc deploy BE. Bật consumer sau khi chạy thử.

**Rollback (API-01):** BE cũ coi thiếu dòng/chỉ in-app là skip.
- Muốn rollback BE khi consumer đang bật, **đặt `NOTIFICATION_RULES_CONSUMER_ENABLED=false` trước** rồi mới đổi binary.
- api và worker dùng chung `.env` (`docker-compose.artifacts.yml`, `env_file: .env`), nên cờ luôn đồng bộ giữa hai tiến trình.
- `shadow_mode_enabled` trong API phản ánh env của api. Shadow chạy ở nơi gửi nhắc (worker, và api nếu endpoint nội bộ gửi).

## Quyết định D-H7-1 (15/10/2026)
- Công ty **chưa lưu** thiết lập: không chặn lịch, không lọc người nhận (chính sách rỗng), cả hai kênh bật.
- Lý do: Q-23 nhằm "không để 18/21 công ty mất nhắc".
  - Áp lịch 30/7/3/0 sẽ bỏ nhắc hạn nộp tự cấu hình ở mốc khác.
  - Áp 3 chính sách sẽ thu hẹp người nhận so với resolver hiện tại.
- Bộ mặc định hiển thị trên UI (lịch 30/7/3/0, 3 chính sách) chỉ có hiệu lực khi admin bấm lưu.
- Có parity test cho kênh/scope/lịch; chính sách người nhận cố ý khác. Không "sửa" code cho khớp UI nếu PO chưa quyết định lại.

## Ma trận test
| ID | Mức | Test |
|---|---|---|
| T1 | evaluator | không có dòng → mặc định, `defaults_applied`, offset 7 được phép |
| T2 | evaluator | dòng `inactive` → `RULE_DISABLED` |
| T3 | evaluator | workflow mốc 1 → được phép (Q-24); hạn nộp mốc 1 → `RULE_SCHEDULE_NOT_MATCHED` |
| T4 | evaluator | `ChannelAllowed` đúng cho: chỉ in-app / chỉ email / cả hai tắt |
| T5 | dispatch | consumer bật, chỉ in-app → không gọi SMTP, 1 in-app, `SENT` |
| T6 | dispatch | consumer bật, tắt in-app → email, không in-app |
| T7 | dispatch | consumer bật, không có dòng → gửi (đổi so với test cũ) |
| T8 | policy | `department_focal` ánh xạ phòng ban công ty (instance trước, global sau); không khớp → không góp |
| T9 | shadow | consumer tắt + shadow → gửi y như cũ, ghi metric/log với outcome; lỗi shadow không đổi kết quả |
| T10 | shadow | consumer bật → không chạy shadow |
| T11 | status API | `dispatch_enforcement_enabled`, `shadow_mode_enabled`, `defaults_applied` |
| T12 | config | đọc được `NOTIFICATION_RULES_SHADOW_ENABLED` |
| T13 | reader | SQL không lọc `status` |
| T14 | FE | banner 3 trạng thái + ghi chú lịch |
| T15 | hồi quy | consumer tắt + shadow tắt → mọi test dispatch cũ pass |
