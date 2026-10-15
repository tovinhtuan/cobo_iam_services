# AUTH-03: tóm tắt

- **Trạng thái:** đã sửa trong code BE (chưa commit, chưa deploy). Finding vẫn mở một phần, chờ OPS-1 và quyết định trần tổng.
- **Kết quả theo endpoint:**
  - Resend OTP: chỉ gửi cho user đang đăng nhập, trần 5/giờ và 10/ngày. Kiểm tra trong tx có lock nên request song song không vượt trần.
  - Quên mật khẩu: trần 3/giờ và 10/ngày theo user, vẫn trả phản hồi chung.
  - nginx: 3 req/phút/IP (burst 3) cho forgot, resend và register.
- **Việc còn lại:**
  1. **OPS-1** (cần user duyệt):
     - chặn `:8080` từ ngoài allowlist, nếu không nginx bị vượt qua;
     - cân nhắc `REGISTRATION_DISABLED=true` trên DEV.
  2. **Quyết định:** trần tổng email auth theo ngày, hoặc tách tài khoản/quota SMTP giữa email auth và nhắc CBTT.
  3. **Follow-up:**
     - index `(user_id, created_at)` cho `password_reset_tokens` (DB-02);
     - retention theo `created_at` (AUTH-20/DB-01);
     - kiểm `time_zone` của production (DB-03);
     - limiter theo IP trong app khi có proxy tin cậy (AUTH-11).
- **Rollback:** revert commit BE và `nginx.conf`. Không có migration; dữ liệu không đổi schema.
