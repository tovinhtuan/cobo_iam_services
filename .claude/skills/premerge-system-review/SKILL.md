---
name: premerge-system-review
description: Cổng kiểm tra cuối trước merge/release cho Cobo - rà regression, security, tenant isolation, contract compatibility, migration, cache, worker idempotency, observability, rollback. Dùng khi feature xong, PR sẵn sàng review hoặc trước khi deploy.
---

# Premerge System Review

## When to use
- A feature or fix is implemented.
- A PR is ready for final review.
- Before a deploy, as input to `deploy-dev-release`.

## Review areas
- Requirement coverage vs spec / ai-cache contract.
- Architectural fit (handler → service → repository; FE vertical slice).
- Validation completeness, body size limits.
- Permission/security, platform vs tenant admin (`cobo-admin-role-guard`).
- API compatibility and mixed-version deploy (`api-compatibility-rolling-deploy`).
- Data consistency, transactions, idempotency, duplicate worker effects.
- Migration added to `migrations/run_dev_migrations.sh` list and reversible or
  documented forward-only.
- Cache effects (Redis effective access, browser bundle) (`cache-versioning-review`).
- UI state completeness.
- Test sufficiency (`-race` for concurrency changes).
- Logging/metrics/debuggability, no secrets in logs.
- Deployment/rollback readiness, feature flags.

## Severity model
- Critical: likely bug, security hole, data corruption, major regression.
- Important: missing edge case, weak validation, maintainability risk, unclear contract.
- Nice-to-have: cleanup, polish, refactor ideas.

## Output format
- Change summary
- Critical findings
- Important findings
- Nice-to-have improvements
- Merge recommendation (merge / merge after fixes / do not merge)
