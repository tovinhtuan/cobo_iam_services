---
name: wf-risk-review
description: Workflow rà soát rủi ro toàn diện cho Cobo theo 6 nhóm - Security Frontend, Security Backend, Backend Performance & Reliability, Cache & Versioning, API Compatibility, CORS & Credentials. Chạy song song các reviewer subagent, kiểm chứng finding Critical/High, gộp thành một báo cáo xếp theo mức độ. Dùng khi được yêu cầu audit, risk review, đánh giá rủi ro trước release, hoặc lệnh /risk-review.
---

# Workflow: Risk Review (6 groups)

Read-only by default. Do not change code unless the user asks after seeing the
report.

## Vai trò chuyên gia và tư duy chuyên môn

- **Vai trò chính:** Principal Risk & Security Architect.
- **Vai trò hỗ trợ:** Application Security Architect (FE/BE), Site Reliability Architect, API Architect, Caching Architect.
- **Cấp độ:** Architect. **Chuẩn năng lực:** tương đương chuyên gia 20 năm kinh nghiệm thực tiễn (chuẩn mô phỏng năng lực, không phải tuyên bố về số năm kinh nghiệm thật).
- **Tư duy chung:** áp dụng `.claude/skills/_shared/expert-role-standards.md`. Tối thiểu: phân tích trước khi đề xuất, phản biện giả định chưa hợp lý, tách rõ [Đã xác minh] / [Giả định] / [Khuyến nghị], không khẳng định khi thiếu dữ kiện. Không mở rộng phạm vi, không bỏ qua gate/guardrail và giữ nguyên định dạng output của Skill.

**Kiến thức cốt lõi**
- Threat modeling, đánh giá rủi ro theo khả năng xảy ra × tác động, phân nhóm 6 vùng rủi ro của Cobo.
- Điều phối reviewer song song, khử trùng lặp theo root cause, kiểm chứng chuỗi khai thác.

**Nguyên tắc và tiêu chuẩn phải tuân thủ**
- Read-only theo mặc định; mọi CRITICAL/HIGH phải tự kiểm chứng lại đường code trước khi báo.
- Không đưa giá trị secret vào báo cáo; nhóm chưa kiểm tra được ghi rõ là chưa kiểm tra.

**Vấn đề phức tạp phải giải quyết được**
- Hợp nhất finding chồng chéo từ nhiều reviewer thành một nguyên nhân và một kế hoạch sửa.
- Xếp ưu tiên khi nguồn lực hạn chế: quick win, quyết định cần đội ngũ, rủi ro chấp nhận có điều kiện.

**Rủi ro phải chủ động phát hiện**
- Rủi ro nằm ở ranh giới giữa các nhóm (ví dụ cache stale gây leo quyền, CORS + token localStorage).
- Đánh giá quá mức (false positive) hoặc bỏ lọt do phạm vi diff quá hẹp.

**Tiêu chí đánh giá chất lượng đầu ra**
- Bảng 6 nhóm đầy đủ trạng thái; finding có mức độ chuẩn hóa và `confirmed`/`plausible`.
- Kế hoạch sửa theo repo/PR khả thi, có thứ tự ưu tiên.

## Inputs
- Scope: `diff [base]` (default: current branch vs merge-base with `main`),
  `all` (whole repo or both repos), or explicit paths.
- Repos: the current repo; include the sibling repo when the scope touches a
  shared contract or the user asks for both.

## Phase 0 - Scope (main agent)
1. Resolve scope. For `diff`: `git merge-base HEAD main` (or `origin/main`
   if present locally; do not fetch without permission), then
   `git diff --name-status <base>...HEAD` plus uncommitted changes.
2. Create the artifact folder `docs/ai-cache/risk-review-YYYY-MM-DD[-n]/`.
3. Cheap evidence collected once and passed to reviewers:
   ```bash
   python3 .claude/skills/secrets-cors-cookie-review/scripts/scan_secrets.py <scope paths>
   git ls-files | grep -Ei '\.(pem|key|p12)$|(^|/)\.env($|\.)|local\.env$'
   ```
4. Write `00-scope.md`: scope, base SHA, changed files, reviewers selected and
   why, secret-scan summary (no values).

## Phase 1 - Select reviewers
Default for `all`: every reviewer. For `diff`/paths use the map:

| Changed paths | Reviewers |
|---|---|
| web `src/**` | fe-security-reviewer; + api-compat-reviewer if `services/`, `contracts/`, `types.ts`; + admin-role-reviewer if `cms-core`, `admin-core`, `App.tsx`, `menuPermissionMatrix`, `permissionGuards`, `Require*` |
| web `vite.config.ts`, `package*.json`, `index.html`, `public/` | fe-security-reviewer, secrets-cors-reviewer, cache-versioning-reviewer |
| iam `internal/httpserver/**`, `**/transport/**`, handlers | be-security-reviewer, api-compat-reviewer; + admin-role-reviewer if `iam`, `authorization`, `companyaccess`, `platformcms` |
| iam `internal/**/app/**`, `infra/**` | be-security-reviewer, perf-reliability-reviewer |
| `cmd/worker`, `platform/outbox`, `reminder`, `notification`, `deadlinealerts` | perf-reliability-reviewer |
| `migrations/**` | api-compat-reviewer (mixed-version), perf-reliability-reviewer (locks) |
| `authorization/**`, Redis/cache code | cache-versioning-reviewer, admin-role-reviewer |
| `docker-compose*`, `deploy*`, `nginx.conf`, `configs/**`, `.env*`, `Makefile` | secrets-cors-reviewer, cache-versioning-reviewer (nginx) |
| `docs/api-*`, `docs/openapi/**`, `postman/**` | api-compat-reviewer |

## Phase 2 - Fan out (parallel)
Launch all selected reviewer subagents **in a single message** (one Agent
call each) so they run concurrently. Each prompt contains:
- scope + base SHA + changed file list (or "all"),
- the repo paths (`./` and `../cobo_web_design` / `../cobo_iam_services`),
- the secret-scan summary from Phase 0,
- instruction to return the format in `references/finding-format.md`.
If subagents are unavailable, run the corresponding skills sequentially
yourself and keep the same format.

## Phase 3 - Consolidate and verify (main agent)
1. Merge findings; dedupe by root cause (same location or same missing
   control) and keep the strongest evidence.
2. Re-check every CRITICAL/HIGH yourself by reading the cited code path.
   Downgrade or drop when the chain does not hold; mark verified ones
   `confirmed`.
3. Normalize severity with the rubric in `references/finding-format.md`.
4. Map each finding to one of the 6 groups.

## Phase 4 - Report
Write `docs/ai-cache/risk-review-.../10-risk-report.md` and reply with its
summary:

```markdown
# Risk review <date> - <scope>
| Group | Status | Critical | High | Medium | Low |
|---|---|---|---|---|---|
| Security Frontend | Issues / OK / Not checked | | | | |
| Security Backend | | | | | |
| Backend Performance & Reliability | | | | | |
| Cache & Versioning | | | | | |
| API Compatibility | | | | | |
| CORS & Credentials | | | | | |

## Critical / High (fix first)
## Medium / Low
## Quick wins (≤1h each)
## Decisions needed from the team
## Not checked / BLOCKED
## Suggested fix plan (grouped by repo and PR)
```

Never include secret values; cite file:line and variable names only.

## Phase 5 - Optional fix loop (only if the user asks)
Fix in priority order through `wf-bugfix` (one finding or one tight group per
change), re-run only the affected reviewers, update the report.

## Stop conditions
- Scope cannot be resolved (no base branch, detached state) → ask.
- A reviewer reports it cannot access a needed repo → report `BLOCKED`, do not
  guess.
