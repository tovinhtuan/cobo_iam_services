---
name: system-design-feature
description: Dùng khi xây tính năng mới từ đầu cho Cobo cần thiết kế xuyên frontend/backend - domain model, state transition, API contract, permission, data flow, failure mode, rollout theo phase và test matrix trước khi code.
---

# System Design Feature

## When to use
- Build a feature from scratch or from a vague request.
- Design a cross-repo feature (API + UI + data model).
- Reduce implementation risk before coding.

## Goal
Turn a vague requirement into an implementable, phased design with guardrails.
Write it with `spec-authoring` when the result should be kept as `SPEC-*.md`.

## Workflow
1. Business objective and success criteria (link to CBTT rule via
   `cobo-domain-cbtt` when relevant).
2. User / operator journey; which role (platform admin, company admin, member).
3. Domain entities, state transitions, invariants.
4. API contract, permission model, validation rules.
5. Frontend slice: routes, screens, hooks, services, UI states, flags.
6. Backend flow: handler, service, repository, cache, outbox/worker, migration.
7. Failure modes: invalid input, partial failure, retry, stale cache, races,
   mixed-version deploy.
8. Rollout plan in small phases (Phase 0 = contract lock).
9. Test matrix and observability points.

## Required checks
- Where is input validated? What is the source of truth?
- Behavior when a dependency is slow or down?
- Idempotency needed?
- Backward compatibility / migration impact?
- Audit / security / tenant isolation concerns?

## Output format
- Objective
- Domain model
- State transitions
- API contract
- Frontend design
- Backend design
- Failure modes
- Rollout plan
- Validation plan
