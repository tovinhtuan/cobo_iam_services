---
name: cobo-task-workflow
description: Điều phối workflow phát triển cho Cobo, tự phân loại task và kết hợp đúng các skill frontend, backend, cross-repo, security, testing và pre-merge theo thứ tự tối thiểu cần thiết.
---

# Cobo Task Workflow

Use this skill as the entry point for any non-trivial Cobo task. It coordinates
repository skills and does not replace their domain instructions.

## Classification and context

Classify the task as analysis/review, UI/UX, frontend feature, backend feature,
cross-repo feature, security, bug fix, or release/deployment. Load the relevant
README, local `AGENTS.md`, `docs/ai-cache/README.md`, focused reusable context,
affected code, tests, contracts, and working-tree status before editing.

For cross-repo work, inspect both repositories. For review-only work, do not
edit code or cache documents unless explicitly requested.

## Skill chains

### Backend feature

```text
backend-feature-delivery
→ backend-api-contract (API changes)
→ backend-authz-session (auth/IAM changes)
→ backend-db-migration-safe (schema/migration changes)
→ backend-worker-idempotent (worker/outbox/retry changes)
→ backend-security-audit
```

### Cross-repository feature

```text
system-design-feature
→ integration-cross-repo
→ backend-feature-delivery
→ frontend skill in the sibling repository
→ cross-repo-security-review
→ backend and frontend tests
```

### Frontend/UI task in the sibling repository

Use the sibling repo's `cobo-task-workflow` with:

```text
frontend-ui-ux-quality
→ taste-frontend-cobo (visual redesign/direction)
→ frontend-feature-slice or frontend-route-screen
→ frontend-test-regression
```

### Security task

```text
backend-security-audit
→ frontend-security-audit (if frontend is affected)
→ cross-repo-security-review (if the boundary spans both repos)
→ focused regression tests
→ premerge-system-review
```

### Bug fix

```text
debugging-and-error-recovery
→ narrow backend/API/authz/migration/worker skill
→ regression test
→ premerge-system-review when risk is material
```

### Release/deployment

```text
premerge-system-review
→ release/deployment gate in the workspace AGENTS.md
→ explicit user authorization for merge, tag, deploy, restart, or push
```

Do not load every skill by default. Add a specialized skill only when its
boundary is affected.

## Execution and verification

- Make the smallest coherent change in the owning repository.
- Re-evaluate the chain if the task expands into auth, migration, worker,
  external integration, or frontend contract work.
- Run relevant checks after each meaningful slice.
- Backend baseline: `go test ./...` and `go vet ./...` when applicable.
- API/Docker baseline: `docker compose -f docker-compose.dev.yml build api`.
- Worker/full-stack changes require relevant Compose checks.
- Cross-repo changes require both frontend and backend baselines.
- Security-sensitive changes require security findings and focused regression
  tests.

If a required check cannot run, report `BLOCKED:` with the exact reason and
still report checks that did run.

## Final review and report

Use `premerge-system-review` for substantial implementation, security fixes,
contract changes, migrations, and cross-repository features. Review correctness,
backward compatibility, authz, tenant isolation, secrets, transactions,
migrations, cache, retries, idempotency, tests, observability, performance,
rollback, and deployment risk.

Report task class, skills applied in order, changed files by repository,
verification results, risks, blocked checks, and next steps. Never claim a
skill, test, browser check, or deployment action that was not completed.
