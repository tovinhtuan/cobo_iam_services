# 13 — Final diff file manifest

## APPLICATION_REQUIRED

```text
internal/deadlinealerts/infra/mysql/list_rows_membership.go
internal/deadlinealerts/infra/mysql/repository.go
internal/deadlinealerts/app/service.go
internal/deadlinealerts/app/status.go
```

## TEST_REQUIRED

```text
internal/deadlinealerts/infra/mysql/list_rows_membership_test.go
internal/deadlinealerts/app/service_test.go
internal/deadlinealerts/app/status_test.go
```

## EVIDENCE_REQUIRED_IF_REPO_POLICY (recommended include)

```text
docs/ai-cache/deadline-alert-v1-phase-1-sql-membership-2026-08-25/
docs/ai-cache/deadline-alert-v1-phase-2-service-integration-2026-08-25/
docs/ai-cache/deadline-alert-v1-phase-3-dev-verification-2026-08-25/00-source-worktree-gate.md
docs/ai-cache/deadline-alert-v1-phase-3-dev-verification-2026-08-25/01-dev-environment-deploy.md
docs/ai-cache/deadline-alert-v1-phase-3-dev-verification-2026-08-25/02-dev-health.md
docs/ai-cache/deadline-alert-v1-phase-3-dev-verification-2026-08-25/03-qa-data-inventory.md
docs/ai-cache/deadline-alert-v1-phase-3-dev-verification-2026-08-25/04-periodic-api-matrix.md
docs/ai-cache/deadline-alert-v1-phase-3-dev-verification-2026-08-25/05-submit-boundary-e2e.md
docs/ai-cache/deadline-alert-v1-phase-3-dev-verification-2026-08-25/06-irregular-regression.md
docs/ai-cache/deadline-alert-v1-phase-3-dev-verification-2026-08-25/07-browser-e2e.md
docs/ai-cache/deadline-alert-v1-phase-3-dev-verification-2026-08-25/08-db-source-parity.md
docs/ai-cache/deadline-alert-v1-phase-3-dev-verification-2026-08-25/09-explain-performance.md
docs/ai-cache/deadline-alert-v1-phase-3-dev-verification-2026-08-25/10-log-review.md
docs/ai-cache/deadline-alert-v1-phase-3-dev-verification-2026-08-25/11-final-diff-gate.md
docs/ai-cache/deadline-alert-v1-phase-3-dev-verification-2026-08-25/12-handoff.md
docs/ai-cache/deadline-alert-v1-phase-3-dev-verification-2026-08-25/api-summary.json
docs/ai-cache/deadline-alert-v1-phase-4-premerge-system-review-2026-08-25/
docs/ai-cache/README.md
```

## FE docs (optional sibling)

```text
cobo_web_design/docs/ai-cache/deadline-alert-v1-phase-1-sql-membership-2026-08-25/00-pointer.md
cobo_web_design/docs/ai-cache/deadline-alert-v1-phase-2-service-integration-2026-08-25/00-pointer.md
cobo_web_design/docs/ai-cache/deadline-alert-v1-phase-3-dev-verification-2026-08-25/00-pointer.md
cobo_web_design/docs/ai-cache/deadline-alert-v1-phase-4-premerge-system-review-2026-08-25/00-pointer.md
cobo_web_design/docs/ai-cache/README.md
```

## EXCLUDE_GENERATED

```text
deploy-artifacts/backend/bin/api — binary
deploy-artifacts/backend/bin/worker — binary
deploy-artifacts/web/dist/** — generated FE bundle
```

## EXCLUDE_SMOKE_SCRIPTS / OPTIONAL ARTIFACTS

```text
docs/ai-cache/deadline-alert-v1-phase-3-dev-verification-2026-08-25/run-api-verify.py — DEV password default
docs/ai-cache/deadline-alert-v1-phase-3-dev-verification-2026-08-25/run-browser-e2e.mjs — DEV password default
docs/ai-cache/deadline-alert-v1-phase-3-dev-verification-2026-08-25/screenshots/** — runtime screenshots (optional)
cobo_web_design/docs/ai-cache/_tmp_dav1_browser.mjs — leftover temp runner
```

## Future staging (DO NOT RUN IN PHASE 4)

```bash
# example only — explicit paths, never git add .
git add \
  internal/deadlinealerts/infra/mysql/list_rows_membership.go \
  internal/deadlinealerts/infra/mysql/list_rows_membership_test.go \
  internal/deadlinealerts/infra/mysql/repository.go \
  internal/deadlinealerts/app/service.go \
  internal/deadlinealerts/app/service_test.go \
  internal/deadlinealerts/app/status.go \
  internal/deadlinealerts/app/status_test.go \
  docs/ai-cache/deadline-alert-v1-phase-1-sql-membership-2026-08-25 \
  docs/ai-cache/deadline-alert-v1-phase-2-service-integration-2026-08-25 \
  docs/ai-cache/deadline-alert-v1-phase-3-dev-verification-2026-08-25 \
  docs/ai-cache/deadline-alert-v1-phase-4-premerge-system-review-2026-08-25 \
  docs/ai-cache/README.md
# then remove smoke scripts/screenshots from index if previously tracked
```
