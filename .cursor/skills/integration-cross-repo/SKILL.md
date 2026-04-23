---
name: integration-cross-repo
description: Dùng khi một feature chạm cả cobo_web_design và cobo_iam_services, để khóa contract, state mapping, error handling và rollout liên repo
---

# Integration Cross Repo

## When to use
Use this skill when:
- frontend and backend change together
- a new API is introduced for web
- backend error model affects frontend UX

## Workflow
1. Xác định contract chung: request, response, errors, auth assumptions.
2. Xác định mapping từ backend states sang frontend UI states.
3. Xác định fallback behavior khi backend chậm/lỗi.
4. Xác định feature flag / phased rollout nếu cần.
5. Xác định test points ở cả 2 repo.
6. Xác định docs/config/env changes.

## Contract checklist
- request fields typed and validated
- nullable fields handled explicitly
- errors mapped to user-meaningful UI
- permission failures distinguishable from generic failures
- loading/retry UX defined

## Output format
- Shared contract
- Frontend mapping
- Backend expectations
- Integration risks
- Validation steps
