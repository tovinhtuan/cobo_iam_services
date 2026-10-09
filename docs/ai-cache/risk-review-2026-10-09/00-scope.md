# Risk review 2026-10-09 — scope

- Scope: `all` (toàn bộ hai repo, không phải diff).
- Repo:
  - `cobo_iam_services` @ `0609fdf` (branch `improve/ui-theme`). Các commit sau review sáng nay (`6205b4d`): `dea4e59 fix bug import` (disclosure template import), `0609fdf add claude skill`.
  - `cobo_web_design` @ `37a799db` (branch `improve/ui-theme`).
- Chế độ: read-only, không sửa code.
- Context đầu vào: `docs/ai-cache/review-production-risk-full-2026-10-09.md`, cộng phần xác minh bổ sung trong cùng phiên (thêm finding `workflowconfig` không có authz, Mailpit/MySQL publish).

## Reviewer được chọn (scope `all` → tất cả)
| Reviewer | Lý do |
|---|---|
| be-security-reviewer | Toàn bộ handler/app/infra IAM |
| admin-role-reviewer | companyaccess, authorization, platformcms, iam; FE cms-core/admin-core |
| perf-reliability-reviewer | worker, outbox, reminder, notification, deadlinealerts, DB pool |
| cache-versioning-reviewer | Redis effective access, cache in-process, nginx Cache-Control, bundle |
| api-compat-reviewer | Contract FE↔BE, migration mixed-version, outbox payload |
| secrets-cors-reviewer | compose, deploy, configs, CORS, git history |
| fe-security-reviewer | cobo_web_design src, vite, bundle |

## Secret scan (chỉ đếm, không có giá trị)
`scan_secrets.py` (heuristic, nhiều false positive):
- IAM: 388 assigned-secret, 31 js-password-literal, 27 mysql-dsn-with-password, 1 private-key.
- Web: 302 assigned-secret, 8 js-password-literal, 3 mysql-dsn-with-password.

Điểm cần reviewer xác minh:
- `configs/login_password_rsa_dev.pem` (private key được track).
- DSN có mật khẩu trong `docker-compose.artifacts.yml:106,129,192` và `docker-compose.dev.yml:70,143`.
- `deploy-dev.ps1:78-79`.
- `deploy-artifacts/t5-*.cjs|sh`, `deploy-artifacts/_tmp_*.py`.
- `cmd/legal-basis-inventory/main.go:136`.
- Web: `src/services/authApi.ts` (nhiều hit, cần xác định đó là tên biến hay giá trị thật).

File được track khớp pattern key/env: IAM `.env.example`, `configs/login_password_rsa_dev.pem`; Web `.env.example`.
