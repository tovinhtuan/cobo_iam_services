---
name: backend-worker-idempotent
description: Dùng khi xây logic worker/job nền trong cmd/worker, tập trung vào idempotency, retry safety, consistency và quan sát được lỗi
---

# Backend Worker Idempotent

## When to use
Use this skill when:
- adding a worker job
- changing retry logic
- processing events/messages/tasks asynchronously

## Workflow
1. Xác định input, trigger, side effects, completion criteria.
2. Xác định idempotency key hoặc equivalent dedupe strategy.
3. Xác định retryable vs non-retryable errors.
4. Xác định partial failure handling.
5. Xác định concurrency/duplicate processing behavior.
6. Xác định logging and metrics for operational visibility.
7. Viết tests cho duplicate execution và retry path.

## Guardrails
- Không assume job chỉ chạy một lần.
- Không thực hiện side effect không có protection khi worker có thể retry.
- Không nuốt lỗi mà không log/context.
- Không để trạng thái trung gian mơ hồ.

## Output format
- Job lifecycle
- Idempotency strategy
- Retry policy
- Failure handling
- Operational signals
