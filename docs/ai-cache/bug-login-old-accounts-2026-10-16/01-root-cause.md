# Bug: "tài khoản cũ không đăng nhập được, không nhận email quên mật khẩu" (DEV, 16/10/2026)

Chỉ điều tra (đọc log nginx/API/worker, DB qua `qa_ro`); chưa sửa code hay dữ liệu.

## Triệu chứng (nginx, UTC; trình duyệt Chrome 153 của người dùng, IP văn phòng)
- 01:15:53, 01:16:00, 01:17:20: `POST /auth/login` → 401 (`INVALID_CREDENTIALS`).
- 01:16:31: `POST /auth/forgot-password` → 200, nhưng không tạo token, không có sự kiện outbox, không có log `password_reset_rate_limited`.
- 01:17:14: `POST /auth/refresh` → 401: phiên cũ trong trình duyệt bị từ chối.

## Root cause
1. **[Đã xác minh] Đăng nhập:**
   - Trình duyệt này từng đăng nhập thành công vào `u_platform_tenant_admin`, `u_admin_dn`, `u_cms_operator` và `d938e010…`.
   - **3 tài khoản đầu nằm trong 15 tài khoản seed bị OPS-1 đổi mật khẩu** (`credentials.password_changed_at` = 15/10 08:21 UTC).
   - Các phiên của chúng trong trình duyệt bị thu hồi với `revoked_reason='ops1_seed_password_rotation'` (4 phiên `u_platform_tenant_admin`, 1 phiên `u_admin_dn`).
   - Mật khẩu cũ → 401; phiên cũ → refresh 401.
   - Không có regression code: form FE chỉ yêu cầu không rỗng; 4.491 lần đăng nhập thành công, gồm các persona sáng nay.
2. **[Đã xác minh] Quên mật khẩu:**
   - Địa chỉ nhập lúc 01:16:31 không khớp `email` hay `login_id` của user nào: `ForgotPassword` trả 200 chung và không làm gì.
   - Kể cả nhập đúng thì 3 tài khoản seed trên cũng không nhận được thư:
     - `u_platform_tenant_admin`: email `@example.com`;
     - `u_cms_operator`: không có email, `login_id` `@example.com`;
     - `u_admin_dn`: `login_id` `@example.com`, email là gmail của người dùng. Gmail này cũng là `login_id` của `d938e010…` (xem BUG-2).
   - Đường gửi thư hoạt động: 3 email reset ngày 15/10 07:38 được worker gửi qua `smtp.gmail.com` (processed, retry 0). DEV gửi thư thật, không qua Mailpit.

## Lỗi thật phát hiện thêm (không gây ra triệu chứng chính)
- **BUG-1 (Medium, quan sát):**
  - FE gửi `email`, không gửi `login_id`.
  - `recordLoginAttempt` và audit `login_failure` chỉ đọc `req.LoginID`, nên từ 20/09 không ghi lần đăng nhập thất bại nào (`login_attempts` success=0 cuối cùng: 20/09), và audit có `login_id` rỗng.
  - Hệ quả: không truy được tài khoản bị lỗi, không phát hiện được dò mật khẩu.
  - Sửa: dùng `loginID` đã chuẩn hoá (`LoginID` hoặc `Email`) trong `internal/iam/app/service.go:436` và `handler.go:120`.
- **BUG-2 (Medium, dữ liệu/logic):**
  - `users.email` không có unique index.
  - `FindUserByEmail` dùng `email = ? OR login_id = ? LIMIT 1` không có `ORDER BY`. Khi một email thuộc nhiều user, user được reset là ngẫu nhiên.
  - DEV có đúng 1 trường hợp (gmail của người dùng: email của `u_admin_dn` và `login_id` của `d938e010…`).
  - Đăng nhập bằng gmail luôn vào `d938e010…`, vì login chỉ so `login_id`.

## Bài học
- OPS-1 đã ghi "script QA cũ sẽ nhận 401", nhưng không kiểm tra tài khoản seed nào đang được người thật dùng hằng ngày.
- Lần sau, trước khi đổi credential: truy `login_attempts`/`sessions` theo user-agent không phải script, rồi báo trước cho người dùng.
