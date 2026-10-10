# Risk review 2026-10-10 - scope: all (lần 4)

- Mode: read-only (`/risk-review all`), không sửa code.
- Repos:
  - `cobo_iam_services` branch `improve/ui-theme` HEAD `1479d72` (merge-base với `main`: `bea9376`)
  - `cobo_web_design` branch `improve/ui-theme` HEAD `453a3494` (không có commit mới từ lần 3)
- Lần trước: `../risk-review-2026-10-10/` (lần 3, iam `ff5ada2`, web `453a3494`). Kế hoạch sửa: `../risk-review-2026-10-10/20-fix-plan.md`.

## Thay đổi code kể từ lần 3
- iam `ff5ada2..1479d72`: 13 commit, 76 file (+4558/-270). Danh sách: `git diff --name-status ff5ada2 1479d72`.
  - `2996f00` PR-A: thu hồi quyền có hiệu lực ngay (generation cache theo company, resolver lọc `membership_status`, invalidate ở mọi mutation, refresh chặn membership inactive).
  - `c6b4ab6` PR-B: chặn leo quyền admin và ghi chéo tenant (ROLE-02/03/05/12/13/14, BES-09).
  - `ae58ecc` RP-10/H16 (transfer ownership atomic), `851f45e` ROLE-07, `512fc04` ROLE-08, `1bf6318` ROLE-25, `4f3a586` ROLE-09/21, `ef58f12` ROLE-10, `48bd3d0` RP-08, `d23fc6f` ROLE-20, `fb0571b` ROLE-24, `c4f2fd9` + `1479d72` ROLE-23 (khoá `GET_LOCK` admin cuối cùng).
  - Vùng chạm: `internal/authorization/**`, `internal/companyaccess/**`, `internal/disclosure/app/cms_template_permissions.go`, `internal/iam/app/service.go`, `internal/workflowconfig/transport/http/authz.go`, `docs/api-contracts-json.md`.
- iam chưa commit: `scripts/devqa/` (mới, `devqa_env.py`, `provision_qa_accounts.py`, `provision_qa_company.py`, `bcrypthash/`, `__pycache__/`), `docs/ai-cache/release-2026-10-10/` (14 thư mục smoke), sửa 4 smoke script ngày 2026-10-09 (bỏ credential gắn cứng), `CLAUDE.md`, 2 skill.
- web chưa commit: `scripts/lib/devQaEnv.mjs`, `docs/ai-cache/dev-qa/` (README + `dev-qa.env.example`), sửa 3 script debug/smoke, `CLAUDE.md`, 3 skill.
- Đã deploy lên DEV theo `../release-2026-10-10/10-role-loop.md` (11 vòng, smoke pass).

## Reviewers (scope `all` → tất cả)
fe-security, be-security, admin-role, perf-reliability, cache-versioning, api-compat, secrets-cors.
Trọng tâm mỗi reviewer: (1) xác minh các bản sửa mới (PR-A, PR-B, vòng ROLE) có giữ không và có gây regression không; (2) cập nhật trạng thái các mục còn mở của lần 3; (3) tìm lỗi mới trên toàn repo.

## Secret scan (không ghi giá trị)
- `01-secret-scan-iam.txt`: 500 match (lần 3: 479). Match mới:
  - 14 smoke script `release-2026-10-10/smoke-*/smoke_dev_*.py` (1 hit mỗi file, dạng `pe***p` — có vẻ là lookup persona), `smoke_dev_platform_onboarding.py` (2), `00-deploy-be.md:46`, `06-release-verify.md:14`.
  - `scripts/devqa/devqa_env.py` (1), `scripts/devqa/provision_qa_accounts.py:108-109` (3).
  - `internal/iam/app/service_refresh_membership_test.go:35` (1, test fixture).
- `01-secret-scan-web.txt`: 310 match (lần 3: 314). Bớt hit ở `scripts/cms-workflow-tab-debug.mjs`, `scripts/sidebar-toggle-verify.mjs`, `smoke-qa-create-type.mjs`; mới: `docs/ai-cache/dev-qa/README.md` (1), `scripts/lib/devQaEnv.mjs` (1).
- File nhạy cảm đang được track: iam `.env.example`, `configs/login_password_rsa_dev.pem` (đã biết từ lần 1); web `.env.example`.
- `.gitignore` iam không có `__pycache__/`, `.playwright-mcp/`; `scripts/devqa/__pycache__/` đang untracked.
