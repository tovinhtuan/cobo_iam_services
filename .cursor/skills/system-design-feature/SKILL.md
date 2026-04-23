---
name: system-design-feature
description: Dùng khi xây một tính năng mới từ đầu, cần thiết kế xuyên frontend/backend, contract, data flow, failure mode và rollout an toàn
---

# System Design Feature

## When to use
Use this skill when the user asks to:
- build a feature from scratch
- design a cross-repo feature
- define API + UI + data model together
- reduce implementation risk before coding

## Goal
Biến yêu cầu mơ hồ thành thiết kế có thể triển khai theo từng bước, có guardrails chống lỗi hệ thống từ đầu.

## Workflow
1. Tóm tắt business objective và success criteria.
2. Xác định user journey / operator journey.
3. Xác định domain entities, state transitions, invariants.
4. Thiết kế API contract, permission model, validation rules.
5. Thiết kế frontend feature slice: routes, screens, hooks, services, UI states.
6. Thiết kế backend flow: handler, usecase, repository, cache, worker nếu có.
7. Liệt kê failure modes: invalid input, partial failure, retry, stale cache, race conditions.
8. Đề xuất rollout plan theo phase nhỏ.
9. Đề xuất test matrix và observability points.

## Required checks
- Input validation nằm ở đâu?
- Source of truth là gì?
- System behavior khi dependency chậm/hỏng?
- Có idempotency cần xử lý không?
- Có backward compatibility / migration impact không?
- Có audit/security concern không?

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
