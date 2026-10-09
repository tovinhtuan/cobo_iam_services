---
name: cobo-task-workflow
description: Entry point điều phối mọi task không tầm thường trong Cobo (cobo_web_design + cobo_iam_services) - phân loại task, chuyển sang workflow wf-* (feature, bugfix, risk-review, pr-review, release) khi phù hợp, hoặc chọn chuỗi skill tối thiểu. Dùng đầu tiên khi bắt đầu feature, bugfix, refactor, audit, deploy hoặc khi chưa rõ nên dùng skill nào.
---

# Cobo Task Workflow

## Purpose

Entry point for any non-trivial Cobo task. It classifies the task and selects
the smallest skill chain. It does not replace the domain skills.

Precedence: explicit user instruction > repo `AGENTS.md`/`CLAUDE.md` >
`docs/ai-cache/README.md` and related ai-cache files > other docs > code.
Do not load every skill by default.

Skill location: each repo has `.claude/skills/<name>/SKILL.md`. Skills tagged
`[web]` exist only in `cobo_web_design`, `[iam]` only in `cobo_iam_services`.
For cross-repo work, read the sibling skill file directly
(`../<sibling>/.claude/skills/<name>/SKILL.md`).

## Step 0: Prefer a workflow skill

If the task matches one of these, switch to it and follow its phases and gates
(they call the specialist skills below in the right order):

| Task | Workflow | Command |
|---|---|---|
| New feature / API / screen | `wf-feature` | `/feature` |
| Bug, regression, incident | `wf-bugfix` | `/bugfix` |
| Audit / risk review (6 risk groups) | `wf-risk-review` | `/risk-review` |
| Review a branch / PR / diff | `wf-pr-review` | `/pr-review` |
| Release readiness / deploy | `wf-release` | `/release-check` |

Reviewer subagents (`.claude/agents/`): `fe-security-reviewer`,
`be-security-reviewer`, `admin-role-reviewer`, `perf-reliability-reviewer`,
`cache-versioning-reviewer`, `api-compat-reviewer`, `secrets-cors-reviewer`.
Launch independent reviewers in one message so they run in parallel.

Otherwise continue with the steps below.

## Step 1: Classify

Pick one primary class: analysis/review, UI/UX, frontend feature, backend
feature, cross-repo feature, domain (CBTT/workflow/deadline), security,
performance/reliability, bug fix, release/deployment, docs/process.
If several apply, use the broadest and add only the needed specialists.

## Step 2: Load context

1. Read README, local `AGENTS.md`/`CLAUDE.md`, `docs/ai-cache/README.md`
   (it may be mojibake-encoded; see `vi-text-encoding`) and the task-relevant
   ai-cache files.
2. Inspect code, tests, routes/contracts, `git status`.
3. Cross-repo: load context from both repos.
4. State current behavior, target behavior, assumptions, affected boundaries,
   verification plan.

Review-only tasks: no code or ai-cache edits unless explicitly requested.

## Step 3: Minimum skill chain

### UI/UX
```text
frontend-ui-ux-quality [web]
→ taste-frontend-cobo [web] (visual direction/redesign only)
→ frontend-ui-state-guard [web]
→ frontend-test-regression [web]
→ e2e-playwright-cobo [web] (when real-browser evidence is needed)
```

### Frontend feature
```text
frontend-feature-slice [web]
→ frontend-route-screen [web] (routing changes)
→ cobo-admin-role-guard (any /cms or /app/admin screen, permission gating)
→ frontend-ui-ux-quality [web]
→ frontend-test-regression [web]
→ frontend-security-audit [web] (data/auth/content boundaries)
```

### Backend feature
```text
backend-feature-delivery [iam]
→ backend-api-contract [iam] (API changes)
→ api-compatibility-rolling-deploy (response/enum/error changes)
→ backend-authz-session [iam] (auth/IAM)
→ cobo-admin-role-guard (platform CMS or tenant admin endpoints)
→ backend-db-migration-safe [iam] (schema)
→ backend-worker-idempotent [iam] (worker/outbox/retry)
→ backend-performance-reliability [iam] (hot paths, concurrency, pools)
→ backend-security-audit [iam]
```

### Domain feature (disclosure, workflow, deadline, notification)
```text
cobo-domain-cbtt (legal rules, terms)
→ backend-workflow-engine [iam] | deadline-reminder-holiday [iam] | notification-delivery [iam]
→ backend-feature-delivery [iam] / frontend-feature-slice [web]
```

### Cross-repository feature
```text
system-design-feature
→ integration-cross-repo
→ api-compatibility-rolling-deploy
→ frontend-feature-slice / frontend-route-screen [web]
→ backend-feature-delivery [iam] (+ api-contract/authz/migration/worker)
→ cross-repo-security-review
→ api-postman-sync
→ frontend-test-regression [web] + Go tests
```

### Security / risk review
```text
frontend-security-audit [web]     # browser boundary
backend-security-audit [iam]      # API/data boundary
secrets-cors-cookie-review        # CORS, cookies, secrets, build-time env
cobo-admin-role-guard             # platform vs tenant admin
cross-repo-security-review        # both boundaries
→ focused regression tests → premerge-system-review
```

### Performance / reliability / cache
```text
backend-performance-reliability [iam]
cache-versioning-review           # browser/CDN/SW and Redis
observability-metrics [iam]       # add signals before/after
```

### Bug fix
```text
bug-triage-debug
→ narrow domain skill
→ regression test (frontend-test-regression [web] / Go test)
→ premerge-system-review (material risk)
```

### Release / deployment
```text
premerge-system-review
→ api-compatibility-rolling-deploy (mixed-version safety)
→ cache-versioning-review (bundle/cache after deploy)
→ deploy-dev-release
→ explicit user authorization for merge/tag/deploy/restart/push
```

### Docs / process
- New spec: `spec-authoring`. After a task: `ai-cache-maintenance`.
- Local environment: `local-dev-stack`. Vietnamese text issues: `vi-text-encoding`.
- Spreadsheet import/export: `excel-import-export` [iam].
- Market/company reference data: `market-reference-integration` [iam].

## Step 4: Execute incrementally

- Smallest coherent change in the owning repo; do not edit the sibling repo
  unless the task or contract requires it.
- Run the narrowest relevant test after each slice.
- Re-evaluate the chain if the change spreads into auth, migration, worker,
  cache, external integration or cross-repo contract.
- No unrelated refactors.

## Step 5: Verification matrix

| Changed area | Baseline |
|---|---|
| Frontend | `npm run lint`, focused `npm test`, `npm run build` |
| Frontend text/content | + `npm run check:mojibake` |
| Backend Go | `go test ./...`, `go vet ./...` |
| Concurrency-sensitive Go | + `go test -race ./<pkg>/...` |
| Backend Docker/API | `docker compose -f docker-compose.dev.yml build api` |
| Worker/full stack | relevant Compose checks (`make dc-up`, worker logs) |
| Migration | migration added to `migrations/run_dev_migrations.sh` list |
| Both repos | both baselines |
| Security-sensitive | findings + focused regression tests |

If a check cannot run, report `BLOCKED: <reason>` and still report the checks
that did run.

## Step 6: Finish

Use `premerge-system-review` for substantial work, security fixes, contract
changes, migrations and cross-repo features. Then `ai-cache-maintenance` if the
repo rules require a task summary. Pre-merge review never replaces tests.

## Completion format

```text
Task class:
Skills applied, in order:
Summary:
Files changed, grouped by repository:
Verification commands and results:
Security/UX/compatibility risks:
BLOCKED checks, if any:
Next steps:
```

Never claim a skill, test, browser check or deployment that was not actually
used or completed.
