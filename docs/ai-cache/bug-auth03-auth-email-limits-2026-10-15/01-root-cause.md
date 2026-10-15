# AUTH-03 (BE): nguyên nhân, cách sửa, kiểm chứng

## Nguyên nhân (đã xác nhận)
1. **Repository:** `issueEmailVerificationOTP` đếm OTP trong 1 giờ rồi gọi `InvalidatePendingEmailVerificationOTPs`. Hàm này `DELETE ... WHERE consumed_at IS NULL`, xoá đúng những dòng cần đếm, nên count luôn ≤ 1.
2. **Service:** `ForgotPassword` không có kiểm tra tần suất nào.
3. **Đồng thời** (review SEC-02, tái hiện được): count → insert không atomic. 20 request song song tạo 20 email.
4. **Resend khi chưa đăng nhập** (review SEC-03): gửi OTP mà không ai dùng được, vì verify OTP bắt buộc có session. Đường này còn lộ user tồn tại qua 429.

## Sửa (không có migration, không đổi mã lỗi)
- `internal/iam/infra/mysql/auth_recovery.go`
  - Invalidate dùng `UPDATE expires_at = now-1s` thay cho DELETE. Dòng vẫn còn để đếm.
  - `IssueEmailVerificationOTPWithinLimits` và `StorePasswordResetTokenWithinLimits` đi qua `issueWithinLimits`. Trong một tx: `SELECT ... FROM users WHERE user_id=? FOR UPDATE`, đếm từng cửa sổ, invalidate/insert, commit. Lock là câu đầu tiên của tx để snapshot RR được tạo sau lock.
  - Có `var _ iamapp.LimitedRecoveryIssuer` để bảo đảm lúc compile.
  - Comment ghi rõ: retention phải xoá theo `created_at` > 24h, không theo `expires_at`.
- `internal/iam/app/contracts.go`: thêm `SendWindow` và interface `LimitedRecoveryIssuer`.
- `internal/iam/app/service.go`
  - Trần:
    - OTP: 5/giờ và 10/24 giờ, trả 429.
    - Quên mật khẩu: 3/giờ và 10/24 giờ. Khi chạm trần vẫn trả **success chung**, không tạo token, không gửi email; `slog.Warn password_reset_rate_limited` chỉ kèm `user_id`.
  - OTP có bước pre-check rẻ trước bcrypt; kiểm tra quyết định nằm trong tx.
  - Repo không implement interface (fake trong test) thì dùng đường cũ.
  - `ResendVerificationEmail` khi không có bearer (chỉ email): trả 200, không gửi.
  - `AdminRequestPasswordReset` không bị giới hạn.
- `deploy-artifacts/web/nginx.conf`: zone `auth_email_per_ip` 3r/m, burst 3, trả 429 cho `/api/v1/auth/(forgot-password|resend-verification-email|register)`. Ba endpoint dùng chung hạn mức theo IP; `/api/` và `login` giữ nguyên.
- Docs: Postman (WebPortal/CMS) đổi request resend sang bearer với body `{}`; `api-v1-implemented-contracts.json` đổi `auth: bearer`.

## Tương thích
- FE chỉ gọi resend không kèm email (`cobo_web_design/src/App.tsx:802`), nên không bị ảnh hưởng.
- Response giữ nguyên dạng `{success:true}` / 429 / 400.
- Deploy lệch phiên bản BE/FE an toàn. nginx đi cùng deploy FE (`deploy-fe` chép `nginx.conf`).

## Kiểm chứng (GATE V)
- **Tái hiện trước khi sửa:**
  - Repo: `OTP sends in the last hour = 1, want 5`.
  - Service: AEL-02, AEL-03, AEL-06 fail.
  - MySQL song song: `20 parallel forgot-password requests: 20 emails`, `20 parallel resends: 20 emails`.
- **Sau khi sửa:**
  - `go test -race ./internal/iam/...` với `MYSQL_TEST_DSN` trỏ vào MySQL 8.0 cục bộ dùng xong bỏ → ok.
  - Test song song chạy 5 lần, mỗi lần đúng 3 token và 5 OTP, chỉ 1 OTP còn dùng được.
  - `go vet ./...` → chỉ còn cảnh báo cũ ở `workflowfulfillment`.
  - `go test ./... -count=1` → 14 test fail, đều có trong baseline HEAD (15), **0 lỗi mới**.
  - `docker compose -f docker-compose.dev.yml build api` → Built.
  - nginx:
    - `nginx -t` (nginx:alpine) → ok;
    - chạy thử với upstream giả: forgot ×6 → `200 200 200 200 429 429`, resend tiếp theo → 429, login ×6 → 200.

## Review
| Reviewer | Kết quả | Xử lý |
|---|---|---|
| security-abuse | SEC-01 High: trần chỉ theo người nhận, tổng lượng email vẫn không có trần | Đã thêm giới hạn theo IP ở nginx. **Còn lại:** chưa có trần tổng theo ngày hoặc tách quota SMTP (cần quyết định); `:8080` public vượt qua nginx (OPS-1) |
| | SEC-02 Medium: song song vượt trần | **Đã sửa** (tx + lock `users`, có test) |
| | SEC-03 Medium: khoá nạn nhân bằng cách dùng hết quota | Resend ẩn danh **đã bỏ**. Quên mật khẩu vẫn khoá được (10 request/ngày cho mỗi nạn nhân): chấp nhận tạm, đường admin reset vẫn còn |
| | SEC-04 Low: 429 lộ tài khoản | **Đã sửa** (resend ẩn danh luôn trả 200) |
| | SEC-05 Low: interface tuỳ chọn có thể âm thầm tắt | **Đã sửa** (assertion lúc compile trong code production) |
| db-tx | DB-01 Low: retention theo `expires_at` sẽ mở lại lỗi | Đã ghi comment trong repo; sẽ thêm vào AUTH-20 |
| | DB-02 Low: thiếu index `(user_id, created_at)` trên `password_reset_tokens` | Follow-up (migration) |
| | DB-03 Low: cửa sổ phụ thuộc session `time_zone` = UTC | Giả định chung của cả repo. DEV đang UTC; cần kiểm tz production |
| | DB-04 Low: có thể deadlock giữa issue và verify OTP của cùng user | Chấp nhận: chỉ chính user gây ra được, lỗi trả 500 rồi thử lại |

## Phần còn lại
- OTP cũ đã bị DELETE trước đây không cần khôi phục.
- Bảng `email_verification_otps` giữ thêm dòng (tối đa khoảng 10/user/ngày khi chưa xác thực). Chưa có retention (AUTH-20).
