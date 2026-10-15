# AUTH-03 (BE): endpoint public gửi email auth gần như không giới hạn

- Nguồn: `docs/ai-cache/risk-review-auth-flows-2026-10-15/10-risk-report.md`, mục AUTH-03 (High).
- Quyết định: user đồng ý sửa ngày 15/10/2026 theo `bugfix-workflow`.
- Hiện tượng:
  - `POST /api/v1/auth/resend-verification-email {email}` (không cần đăng nhập) gửi được OTP không giới hạn, vì giới hạn "5/giờ" không bao giờ kích hoạt.
  - `POST /api/v1/auth/forgot-password` không có giới hạn nào.
  - DEV gửi qua Gmail (`SMTP_HOST=smtp.gmail.com`). Hết quota thì email reset, mời, OTP và nhắc CBTT của mọi công ty đều lỗi.
- Kỳ vọng: mỗi người nhận có trần theo giờ và theo ngày; vượt trần thì không tạo token, không gửi email.

## Tái hiện (GATE R)
- Repo, MySQL 8.0 cục bộ dùng xong bỏ (container `cobo-authrl-test`, chỉ có migration 0001/0007/0027, không đụng DEV):
  - test `TestAuthRecoveryLimits_OTPSendsStayCountedAfterInvalidation`, chạy 5 lần (invalidate rồi store);
  - trước khi sửa: **fail** `OTP sends in the last hour = 1, want 5`.
- Service:
  - `TestAuthEmailLimits_ForgotPassword`: AEL-02 (trần giờ) và AEL-03 (trần ngày) **fail** (`token stored=1 emails published=1`);
  - `TestAuthEmailLimits_ResendVerificationOTP`: AEL-06 (trần ngày) **fail** (`error = <nil>, want 429`).
