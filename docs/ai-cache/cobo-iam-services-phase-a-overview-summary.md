# Cobo IAM Services - Phase A Overview Summary

## Scope
- Repository analyzed: `cobo_iam_services`
- Objective: produce Phase A architecture and rollout plan for modular-monolith IAM/authorization platform.

## Current State
- Repository is empty (no source files, no configs, no migrations, no runtime bootstrap).
- No existing auth/authz implementation to reuse.
- No existing database schema or migration history.

## Target Direction
- Build modular monolith in Go with two processes: `api` and `worker`.
- MySQL 8.0 as transactional store.
- Outbox pattern for async notification/access projection tasks.
- Tenant isolation based on `company_id` and authorization principal `membership_id`.

## Recommended Incremental Rollout
- P0: project skeleton, platform shared libs, IAM + CompanyAccess core, Authorization core, Audit, Outbox, foundational APIs.
- P1: Disclosure + Workflow + Notification module skeleton and core use cases.
- P2: access projection optimization, caching, SSO/MFA hooks.

## Risk Focus
- Highest: tenant-boundary enforcement gaps, inconsistent authorization checks, migration safety.
- Mitigation: explicit authz contract, per-request company context checks, small reversible migrations, audit-first for sensitive actions.

## First Implementation Order
1. Bootstrap (`go.mod`, `cmd/api`, `cmd/worker`, shared config/logger/db/tx).
2. Core schema migration batch 0001 (IAM, company access, authz base, audit, outbox, idempotency).
3. IAM login + memberships + select/switch company flows.
4. Internal authorizer contract + `/internal/v1/authorize`.
5. Effective access API and baseline admin APIs for memberships/roles/departments/titles.

## Detailed Implementation Doc
- See: `docs/implementation-step-by-step.md`
- API JSON examples: `docs/api-contracts-json.md`

