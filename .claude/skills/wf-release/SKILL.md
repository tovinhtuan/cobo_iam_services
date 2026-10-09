---
name: wf-release
description: Workflow release/deploy Cobo lên dev server có cổng kiểm soát - xác định SHA, chạy toàn bộ check, risk review theo diff, ma trận tương thích FE/BE, migration, cache/bundle, xin phép rõ ràng rồi mới deploy, kiểm tra sau deploy, rollback và báo cáo. Dùng khi được yêu cầu release, deploy, kiểm tra trước release, hoặc lệnh /release-check.
---

# Workflow: Release / Deploy

`/release-check` runs Phases 0-4 only (no deploy). Deploy (Phase 5) needs an
explicit user instruction naming the target and mode in this conversation.

## Phase 0 - Candidate → `docs/ai-cache/release-YYYY-MM-DD/00-candidate.md`
For each repo: branch, HEAD SHA, clean/dirty working tree (list dirty files),
previous deployed SHA if known (ai-cache, server `schema_migrations`, tags).

## Phase 1 - Checks
| Repo | Commands |
|---|---|
| iam | `go vet ./...`, `go test ./...`, `docker compose -f docker-compose.dev.yml build api` |
| web | `npm run lint`, `npm test`, `npm run build`, `npm run check:mojibake` |
Deploy scripts do not run tests; these are mandatory. Failures stop the
release unless the user explicitly accepts them.

## Phase 2 - Risk review of the release diff
Run `wf-risk-review` with scope `diff <previous-deployed-SHA>` (or the agreed
base). CRITICAL/HIGH must be fixed or explicitly accepted.

## Phase 3 - Release-specific checks → `03-release-checks.md`
- Compatibility matrix (`api-compatibility-rolling-deploy`): old FE ↔ new BE,
  new FE ↔ old BE, old worker ↔ new schema; deploy order.
- Migrations: every new file listed in `migrations/run_dev_migrations.sh`,
  forward-only ones flagged, lock-heavy ones scheduled.
- Env/flags: new variables present in server `.env`, compose files, and
  required `VITE_*` flags (`ensure-dev-fe-vite-flags`).
- Cache: index.html no-cache, hashed assets, old chunks handling
  (`cache-versioning-review`).
- Secrets: scan of the release diff clean (`secrets-cors-cookie-review`).
- Rollback point: previous binary/dist or SHA to rebuild; DB rollback or
  forward-fix plan.

## Phase 4 - Go / No-go  **GATE D**
Reply with the go/no-go table and wait:
```text
Repos/SHAs:
Checks: pass/fail/BLOCKED
Risk review: Critical/High open?
Compatibility & deploy order:
Migrations:
Env/flags:
Rollback point:
Recommendation: GO / NO-GO (reasons)
```

## Phase 5 - Deploy (explicit approval only)
Follow `deploy-dev-release`: migrations → BE → verify → FE → verify.
Use `make deploy-dev MODE=...` (or the specific targets). Never deploy a SHA
other than the one approved; stop if HEAD moved.

## Phase 6 - Post-deploy → `06-post-deploy.md`
`/healthz`, `/readyz`, header check script, `make dev-ps`, `make dev-logs`
(panics, migration errors, outbox failures), short E2E smoke for changed
flows. On failure: stop, report, propose rollback; do not improvise fixes on
the server.

## Phase 7 - Close
ai-cache summary, CHANGELOG if kept, follow-ups.
