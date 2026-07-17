# Phase 4 — Login/Refresh Rate Limit

## Status
`PLAN_ACCEPTED_FOLLOW_UP`

## Current cycle decision
- High blocker COBO-SEC-001 prioritized and fixed first.
- No limiter landed in this patchset to avoid broad auth regression risk in same hotfix cycle.

## Accepted implementation plan (next patch, dated)
- Target endpoints:
  - `POST /api/v1/auth/login`
  - `POST /api/v1/auth/refresh`
  - optional extension: forgot/reset/verify-email resend.
- Design:
  - middleware with key dimensions:
    - login: by `IP + login_id`
    - refresh: by `IP + session/user`
  - configurable thresholds via env.
  - return `429` with stable error code.
  - never expose account existence.
- Target delivery date: **2026-07-19** (DEV branch).

## Risk note
- Medium finding remains open but explicitly tracked with committed plan and date.
