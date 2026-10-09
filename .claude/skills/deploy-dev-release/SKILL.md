---
name: deploy-dev-release
description: Dùng khi chuẩn bị hoặc thực hiện deploy/release Cobo lên dev server - make deploy-dev (MODE=be|fe|all|migrate|verify), deploy-dev.sh/ps1, docker-compose.artifacts.yml, push migration, verify healthz/readyz, rollback, CHANGELOG/handoff. Chỉ chạy deploy khi người dùng yêu cầu rõ ràng.
---

# Deploy Dev & Release

Merge, push, tag, deploy, restart and post-merge fetch require an **explicit
user request** for that exact operation (see workspace `AGENTS.md`).

## Mechanics (verify; as of 2026-10; run from `cobo_iam_services`)
- `make deploy-dev MODE=be|fe|all|migrate|verify` → `deploy-dev.sh`
  (Windows: `make deploy-dev-win` → `deploy-dev.ps1`, plink/pscp, credentials
  from gitignored `deploy-dev.local.env`).
- Steps: SSH preflight → `go build` / `npm run lint` → diff local migration
  list vs server `schema_migrations` → `push-migration.sh` → `make deploy-be`
  / `deploy-fe` → verify `/healthz`, `/readyz`, nginx `/api`.
- `deploy-be`: `be-build-linux` (CGO off) → copy `bin/` + `configs/` →
  restart api + worker. `deploy-fe`: build with required `VITE_*` flags
  (`ensure-dev-fe-vite-flags`) → copy `dist` + `nginx.conf` → recreate web
  only (never api).
- Server stack: `docker-compose.artifacts.yml` (nginx web, api, worker, MySQL,
  Mailpit; `ENV=development`).
- Helpers: `make dev-ps|dev-logs|dev-restart|dev-ssh|dev-fix-web-perms`,
  `make push-migration FILE=NNNN_x.up.sql`.
- Note: deploy scripts do not run `go test` - run tests yourself first.

## Release gate (before any deploy)
1. Exact candidate commit SHA for each repo; clean working tree or list of
   uncommitted files explicitly accepted by the user.
2. Tests: `go test ./...`, `go vet ./...`, `npm run lint`, `npm test`,
   `npm run build`, `npm run check:mojibake`.
3. `premerge-system-review` done; compatibility matrix from
   `api-compatibility-rolling-deploy` for API changes.
4. Migrations: every new file is in `migrations/run_dev_migrations.sh`;
   order checked; forward-only ones flagged.
5. Env/flags: new env vars present on server `.env` and compose; `VITE_*`
   flags set for the FE build.
6. Secrets: scan (`secrets-cors-cookie-review`) clean for changed files.
7. Rollback plan: previous binary/dist kept or rebuildable from previous SHA;
   down migration or forward fix documented.

## Deploy order
Migrations (expand) → BE (api + worker) → verify → FE → verify → contract
cleanup in a later release.

## Post-deploy verification
```bash
curl -fsS http://<host>:3000/healthz && curl -fsS http://<host>:3000/readyz
bash .claude/skills/cache-versioning-review/scripts/check_http_headers.sh http://<host>:3000
make dev-ps && make dev-logs   # look for panics, migration errors, outbox failures
```
Plus a short smoke via `e2e-playwright-cobo` (web repo) when UI changed.

## Report
```text
Repos and SHAs deployed:
Mode / targets:
Migrations applied:
Checks before deploy:
Post-deploy checks:
Rollback point:
Issues / follow-ups:
```
Record the summary in ai-cache (`ai-cache-maintenance`) and CHANGELOG if the
repo keeps one.
