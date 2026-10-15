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

## Deploy DEV 2026-10-15
- BE `d74c983` (docs `db2e5ee`): api + worker được recreate; `/healthz` và `/readyz` OK.
  - **Không có bản sao binary BE cũ.** Rollback bằng cách build lại từ `bf1b64b` (`make be-build-linux`) rồi deploy lại.
- FE `bb263dfe` (bundle `index-B2g83lTQ.js`, kèm `nginx.conf` mới). Rollback: `web/releases/pre-auth01-auth03-20261015T073547Z.tar.gz` (gồm `dist` + `nginx.conf`).
- Smoke DEV `refs/smoke_auth03_dev.py` → 6/6 (`refs/smoke-auth03-dev-result.json`):
  - resend khi chưa đăng nhập: 200, không tạo OTP;
  - quên mật khẩu ×4 cho persona USER: lần đầu phát 3 token, lần chạy lại phát 0, log `password_reset_rate_limited` có 5 dòng;
  - nginx: `[200, 200, 200, 429, 429]` (IP đã có bucket được burst 3);
  - login không bị ảnh hưởng.
- Tác dụng phụ trên DEV: 3 email reset gửi tới địa chỉ của persona USER (token hết hạn sau 30 phút, không dùng).
- Log api/worker 20 phút sau deploy: 0 dòng panic/ERROR.
