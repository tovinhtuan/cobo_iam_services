# Risk review 2026-10-15 — luồng đăng ký / đăng nhập / quên mật khẩu — phạm vi

- Mode: read-only (`/risk-review rà soát flow đăng ký/đăng nhập/quên mật khẩu`). Không sửa code, không ghi DB, không thao tác server.
- Skill: `backend-production-review` (risk-review mode).
- Phạm vi theo **luồng** (không theo diff): code tại HEAD của hai repo.
  - `cobo_iam_services` branch `improve/ui-theme` HEAD `bf1b64b`; merge-base với `origin/main` = `bea9376` (không có `origin/develop`). 467 commit kể từ base.
  - `cobo_web_design` HEAD `c1e57037`; merge-base với `origin/main` = `8d98b6eb`.
- Endpoint (`internal/iam/transport/http/handler.go:35-47`), tất cả trừ select/switch-company đều không cần đăng nhập:
  - `GET /api/v1/auth/login-password-key`;
  - `POST /api/v1/auth/{register, login, refresh, logout, forgot-password, resend-verification-email, reset-password, verify-email, select-company, switch-company}`;
  - `GET /api/v1/auth/invitations/validate`, `POST /api/v1/auth/invitations/accept`.
- Code chính BE:
  - `internal/iam/app/service.go`, `contracts.go`, `hooks.go`;
  - `internal/iam/infra/mysql/{auth_recovery,credentials,login_attempts,sessions,user_invitation_store,invitation_mysql}.go`;
  - `internal/iam/loginpassword/`, `internal/iam/registrationmysql/`, `internal/iam/infra/token/*`;
  - nginx `deploy-artifacts/web/nginx.conf`.
- Code chính FE:
  - `src/pages/public/{Login,Register,ForgotPassword,ResetPassword,VerifyEmailToken,AcceptInvitation,CompanySelection}Page.tsx`, `loginPasswordCrypto.ts`, `loginErrorMessage.ts`;
  - `src/services/authApi.ts`, `src/App.tsx`.

## Reviewer (chạy song song, read-only)
| Reviewer | Vì sao |
|---|---|
| `be-security-reviewer` | authn/authz, token, session, reset/verify, enumeration |
| `security-abuse-reviewer` | endpoint public: brute force, stuffing, spam email, dò email, DoS RSA/bcrypt |
| `db-tx-reviewer` | đăng ký nhiều bảng, dùng token một lần, đếm khoá, trùng email |
| `idempotency-reviewer` | email/outbox, retry đăng ký/quên mật khẩu |
| `resilience-observability-reviewer` | log lộ dữ liệu nhạy cảm, timeout SMTP, audit/metric |
| `concurrency-distributed-reviewer` | rate limit/khoá trong tiến trình, refresh đồng thời, nhiều replica |
| `fe-security-reviewer` | redirect, token trong URL, localStorage, mật khẩu dạng rõ |
| `secrets-cors-reviewer` | khoá RSA dev trong repo, secret JWT mặc định, CORS, link reset |

## Kiểm tra đã chạy (chỉ đọc)
- `go vet ./internal/iam/...` → rc=0.
- `go test ./internal/iam/... -count=1` → 8 package ok.
- `staticcheck ./internal/iam/...` → 1 cảnh báo U1000 trong test (`me_handler_me_test.go:28`, field không dùng).
- `diff_hygiene.py bea9376` (BE): BLOCKERS 89, WARNINGS 302.
  - Phần lớn là file `.out`/`.log` smoke trong `docs/ai-cache`, `.playwright-mcp/` (có từ trước, xem SEC-13 của review 10-09).
  - **Liên quan luồng:** `configs/login_password_rsa_dev.pem` (key material) — xem `10-risk-report.md`.
- `diff_hygiene.py 8d98b6eb` (FE): BLOCKERS 9, WARNINGS 1.317 (ảnh/mp4/`__pycache__` trong `docs/`; không liên quan auth).
- `scan_secrets.py` (giá trị đã che):
  - BE: 1.570 dòng, phần lớn trong test/worktree `.claude/worktrees/*`/docs smoke. Liên quan luồng: khoá RSA riêng `configs/login_password_rsa_dev.pem` (đã xác minh: commit `378faaa`, `Makefile:152` chép lên DEV, `docker-compose.artifacts.yml:130` dùng nó).
  - FE `src/`: 214 lần trúng mẫu `assigned-secret` (gán biến tên password/token), chưa thấy giá trị thật.

## Phát hiện cũ cần theo dõi
BES-42, BES-04/OBX-04/SEC-29, BES-03, BES-06, BES-33, FES-10, FES-11, FES-12, dò email qua thời gian phản hồi (risk-review-2026-10-09…10-10-3).
