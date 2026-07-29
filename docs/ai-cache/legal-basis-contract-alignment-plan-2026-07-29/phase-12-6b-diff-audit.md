# Phase 12.6B-Plan — Diff audit

## Allowed in this phase

- Docs / evidence under `docs/ai-cache/legal-basis-contract-alignment-plan-2026-07-29/phase-12-6b-*`
- 12.6A closure docs (`phase-12-6a-scope-exception.md`, updated handoff)
- Append-only updates to plan index files (08/10/17/18, plan-results, README, reusable)

## Forbidden (must remain absent)

- `cmd/legal-basis-backfill` or `--apply` implementation
- Migration SQL / 0122 apply
- UPDATE/INSERT legal basis data
- docker-compose / Dockerfile changes
- Runtime API / CMS / tenant UI changes
- Deploy scripts

## Note on pre-existing dirty tree

Uncommitted 12.6A inventory tooling (`sql_allowlist*`, `cmd/legal-basis-inventory`) may still exist from prior cycle — **out of 12.6B-Plan commit scope** unless separately approved as docs/tool chore. This plan's deliverable is documentation.

## Verification performed

- RO inventory refresh matched allowlist (mutations 0)
- No `docker compose build` during 12.6B-Plan
- Secret scan expected: no DSN/password/full legal text in new files
