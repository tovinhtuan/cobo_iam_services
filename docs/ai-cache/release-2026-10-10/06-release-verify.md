# Checks, risk review, deploy, smoke — release DEV 2026-10-10 (lần 2)

## Phase 1 — Checks (IAM; FE không đổi `src/` nên không chạy npm)
- **`go build ./...`:** OK.
- **`go vet ./...`:** chỉ còn lỗi có sẵn: `workflowfulfillment/required_document_gate_test.go:326-327` copylocks (PERF-25).
- **`go test ./...`:** 80 package ok. 16 test fail, **giống hệt tập fail ở HEAD gốc `ff5ada2`** (chạy với các thay đổi đã stash), nên là fail có sẵn, không chặn.
  - `companyaccess`: `TestUpdateNotificationRule_TierEnforcement_FlagOffAllowsPremium`, `TestCreateSelfServiceCompany_FeatureFlagOff` (PERF-18).
  - `disclosure/app` ×6, `legal_basis_backfill` ×1, `httpserver` ×5 integration, `notification/app` `TestContract_VariableParity`, `platform/config` `TestLoad_UserAvatarEnvOverride`.
  - Ghi chú: các báo cáo trước trong ngày ghi "11 test" là đếm nhầm; con số đúng là 16.
- **Docker build:** **BLOCKED** (Docker daemon local không chạy).

## Phase 2 — Risk review của diff (so với `c6b4ab6` / `453a3494`)
Diff chỉ gồm script, docs và skill.
- **Secret scan:** 8 hit, đều là false positive (SQL `VALUES (...)` dùng biến; locator Playwright; `email, password = persona(p)`).
- **Smoke script:** diff **gỡ bỏ** IP và mật khẩu gắn cứng khỏi smoke script IAM và 3 script web; chuyển sang loader `~/.cobo/dev-qa.env`. Đây là một phần của SEC-07/H21.
- **Script provision QA:** `provision_qa_accounts.py` ghi DB DEV bằng root qua SSH. Đây là tool chủ động chạy tay, không chạy trong deploy. Persona có id cố định `m_qa_*`/`u_qa_persona_*`. DB user `qa_ro`/`qa_rw` chỉ là `@localhost` (không qua port 3306 public).
- **Kết luận:** không có CRITICAL/HIGH mới.

## Phase 3/4 — Release checks, Go/No-go
- **Migration:** không có (DEV 148 = local 148).
- **Env/flags:** không có biến mới.
- **FE:** không đổi `src/`.
- **Tương thích:** không đổi so với lần deploy 04:30Z.
- **Rollback point:** `bin/{api,worker}.rollback.20261010T042845Z` (không đổi).
- **Quyết định:** GO với mode `verify` (không build lại vì không có code mới).

## Phase 5 — Deploy
`deploy-dev.ps1 -Mode verify`:
- SSH OK.
- Container: api và worker Up 4h (bản 04:30Z), web, MySQL, Redis healthy.
- `/healthz` ok, `/readyz` ready.
- SHA binary trên server không đổi (api `fd52e46f…`, worker `54525832…`).

## Phase 6 — Smoke tự động
Script `smoke-pr-ab/smoke_dev_pr_ab.py`, output `smoke-pr-ab/smoke_dev_pr_ab.out`. Kết quả **29/29 PASS**.
- **Persona:** ENT, ENT2, MEMBER, CMS, USER (`qa.persona.*@cobo.test`). Chỉ ghi trên membership QA (`m_qa_*`). Các thay đổi đều đã khôi phục và được kiểm bằng `run_sql` (qa_ro).
- **Happy path ENT:**
  - Effective access có `rbac.manage` (33 quyền).
  - List roles và memberships trả 200.
  - `invite-roles` có `admin_doanh_nghiep` (ROLE-03: admin không bị giới hạn).
  - CMS persona vào được platform CMS.
- **MEMBER:** `/admin/roles`, `/admin/invite-roles`, PATCH membership → 403 `PERMISSION_DENIED`.
- **Company khác (c_002):** list memberships → 403 `COMPANY_SCOPE_MISMATCH`; PATCH membership và DELETE role → 404 `MEMBERSHIP_NOT_FOUND`.
- **BES-12:** status `suspended` → 400.
- **BES-21 / ROLE-13:** ENT khoá hoặc xoá membership CMS operator, gỡ role `cms_operator`, gỡ `platform.cms.view` → đều 403. CMS vẫn vào được `/cms`.
- **H3 / H17 / CACHE-11:**
  - MEMBER có 8 quyền (cache đã được nạp).
  - ENT khoá MEMBER → effective access còn **0** quyền ngay lập tức. DB ghi `inactive`. Refresh → 401 `SESSION_EXPIRED`.
  - Mở khoá → MEMBER có lại 8 quyền.
- **CACHE:** 4 key `effective_access*:v2` trên Redis.
- **Cleanup:**
  - 5 membership QA về đúng trạng thái ban đầu (đều `active`).
  - CMS vẫn giữ role `cms_operator`.
  - Không còn approval pending.
- **Log api trong 15 phút quanh smoke:** 0 panic/ERROR, 0 response 5xx, 0 lỗi invalidate cache.

**Không smoke được trên DEV** (đã có unit test/integration test; ghi lại để biết):
- **ROLE-05:** primary admin là tài khoản thật, không phải dữ liệu QA.
- **ROLE-02:** `qa_rw` không có quyền dọn `resource_scope_rules`. Audit trước đó A4 = 0.
- **ROLE-12 / BES-18:** DEV chưa có rule `alert_channel_prefs`.
  - BES-18 (HIGH, chỉ lộ ra trên MySQL) **vẫn chưa được kiểm trên MySQL thật**.
  - Đề xuất: chạy `TestIntegration_ApproveNotificationPrefsPatch_WritesRule` với `MYSQL_TEST_DSN` trỏ vào DB test riêng, hoặc tạo rule prefs cho một company QA riêng rồi smoke.
- **ROLE-14 (break-glass):** chưa smoke.

## Follow-up
- Kiểm BES-18 trên MySQL (như trên).
- FE PR-F: API-17/18 (`suppressForbiddenNavigation`, hiển thị `permission_codes`, ẩn nút duyệt break-glass cho target).
- Docker build: chạy lại khi Docker daemon bật.
