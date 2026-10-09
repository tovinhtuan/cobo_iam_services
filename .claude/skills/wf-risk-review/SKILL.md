---
name: wf-risk-review
description: Workflow rà soát rủi ro toàn diện cho Cobo theo 6 nhóm - Security Frontend, Security Backend, Backend Performance & Reliability, Cache & Versioning, API Compatibility, CORS & Credentials. Chạy song song các reviewer subagent, kiểm chứng finding Critical/High, gộp thành một báo cáo xếp theo mức độ. Dùng khi được yêu cầu audit, risk review, đánh giá rủi ro trước release, hoặc lệnh /risk-review.
---

# Workflow: Risk Review (6 groups)

Read-only by default. Do not change code unless the user asks after seeing the
report.

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
