---
name: backend-worker-idempotent
description: Dùng khi xây/sửa job nền trong cmd/worker của cobo_iam_services - outbox processor, reminder dispatch, periodic seeding, email - đảm bảo idempotency, retry/backoff, SKIP LOCKED, không xử lý trùng, không overlap, quan sát được lỗi.
---

# Backend Worker Idempotent

## When to use
- Add a worker step or outbox event handler.
- Change retry/backoff, claiming or dispatch logic.

## Project facts (verify)
- `cmd/worker/main.go`: one goroutine, ticker `WORKER_TICK_INTERVAL` (5s),
  steps run sequentially: outbox reaper → periodic seed/materialize → reminder
  seed/materialize/dispatch (limit 50) + stale DISPATCHING reaper → workflow
  department email (flagged) → `outbox.Processor.Tick` (batch 50).
- Outbox (`internal/platform/outbox`): claim with `FOR UPDATE SKIP LOCKED`,
  exponential backoff 1-32s + jitter or handler `RetryAt`, 10 failures →
  `failed_permanent`. Unknown event types are marked processed (silently
  dropped) - register handlers before emitting new event types.
- Reminders: per-row `FOR UPDATE`, statuses PENDING → DISPATCHING → SENT /
  RETRY_SCHEDULED / FAILED, deterministic `idempotency_key` + `INSERT IGNORE`.
- Email dedupe via unique `uk_email_notifications_idempotency`.
- Fire-and-forget goroutines exist (in-app notifications) - avoid adding more;
  never use a request context in a detached goroutine.

## Workflow
1. Input, trigger, side effects, completion criteria.
2. Idempotency key / dedupe strategy (unique key, `INSERT IGNORE`, status guard
   `WHERE status=?`).
3. Retryable vs non-retryable errors; backoff with jitter; max attempts.
4. Partial failure handling; visibility timeout / stale-claim reaper.
5. Concurrency: assume ≥2 worker replicas and overlapping ticks.
6. Timeouts on every external call (SMTP, HTTP).
7. Logs + metrics (`cobo_*`) for backlog, failures, stuck rows.
8. Tests for duplicate execution, retry path, crash after side effect.

## Guardrails
- Never assume a job runs once.
- No unprotected side effect on a retriable path.
- Never swallow errors without context.
- No ambiguous intermediate state.
- Never log full payloads that may contain PII.

## Output format
- Job lifecycle
- Idempotency strategy
- Retry policy
- Failure handling
- Operational signals
