---
name: backend-performance-reliability
description: Dùng khi review hoặc sửa hiệu năng/độ tin cậy cobo_iam_services (Go) - race condition, deadlock, goroutine/memory leak, OOM, connection pool exhaustion MySQL/Redis, slow query, thiếu timeout, retry storm, cache stampede, background job overlap, duplicate processing, idempotency, bottleneck khi tải đồng thời. Gồm go test -race, pprof, EXPLAIN, load test local.
---

# Backend Performance & Reliability

## When to use
- Hot endpoints (login, `/me*`, effective access, dashboard, lists, exports).
- Worker/outbox/reminder changes, new goroutines, new caches.
- New external calls (SMTP, HTTP, vnstock DB), uploads, large queries.
- Incidents: latency, 5xx, OOM, stuck jobs, DB "too many connections".

Load tests run **only** against local Compose or an environment the user
explicitly names. Never against shared/prod without authorization.

## Current state (verify; as of 2026-10)
- MySQL pool (`internal/platform/db/mysql.go`): MaxOpen 25, MaxIdle 10,
  lifetime 30m, idle 5m. Second pool for `VNSTOCK_MYSQL_DSN`.
- Request context passed down; no per-query timeouts. HTTP server timeouts
  from env (read/write 15s, idle 60s, read-header 5s).
- SMTP `smtp.SendMail` without deadline (notification adapter, binding mailer,
  worker auth mail); ad-hoc approval emails sent synchronously in API.
- Opaque access tokens in an unbounded in-process map (memory growth).
- Fire-and-forget goroutines (`reminder/app/service.go`, `iam/app/service.go`),
  one using a request context that may already be cancelled.
- Worker: single goroutine, sequential steps every 5s; one slow step delays
  all. Outbox reaper keyed on `available_at` → possible double processing
  with >1 worker replica.
- Unbounded caches: workflow-override preview cache; holiday cache caches errors.
- Company switch triggers ~10-15 parallel `/me*` calls (nginx limits tuned for it).
- `go test -race` not used anywhere.

## Review checklist

Concurrency
- [ ] Shared maps/slices guarded (`sync.Mutex`/`sync.Map`) - run `-race`.
- [ ] Every goroutine has an owner, a stop condition and uses a context
      derived from a long-lived parent, not `r.Context()` after return.
- [ ] Bounded concurrency (worker pool / semaphore) for fan-out.
- [ ] Lock ordering consistent; no lock held across I/O.

Database
- [ ] `rows.Close()` deferred, `rows.Err()` checked, tx always committed or
      rolled back (connection leak → pool exhaustion).
- [ ] No N+1 in list endpoints; index exists for every WHERE/ORDER BY on
      large tables (`EXPLAIN`).
- [ ] Transactions short; no external call inside a tx.
- [ ] Deadlock-prone patterns: same rows locked in different orders;
      `SELECT ... FOR UPDATE` without index → gap locks. Retry deadlock errors
      (MySQL 1213) idempotently.
- [ ] Per-call timeout for expensive queries (`context.WithTimeout`).

External calls
- [ ] Timeout on every SMTP/HTTP call (dialer + overall deadline).
- [ ] Retries: bounded attempts, exponential backoff **with jitter**, only
      for retryable errors, idempotent operation.
- [ ] No retry at multiple layers at once (retry storm); FE refresh is
      single-flight - keep it that way.

Memory
- [ ] Bounded caches (size + TTL); no per-request growth of global maps.
- [ ] Streaming for large exports/uploads; `http.MaxBytesReader` on bodies.
- [ ] Excel/CSV generation not fully buffered when large.

Worker / jobs
- [ ] Safe with 2+ replicas and overlapping ticks (SKIP LOCKED / status guard
      / unique key) - see `backend-worker-idempotent`.
- [ ] Slow step can't starve others (time budget per step or separate loops).
- [ ] Stuck-row reaper and metrics (`cobo_outbox_stale_processing`,
      `cobo_reminder_stuck_dispatching`).

Cache
- [ ] Stampede protection on hot keys (jittered TTL, coalescing).
- [ ] Fallback when Redis is down does not overload MySQL.

## Commands
```bash
go test -race ./internal/<pkg>/...            # race detector (focused first)
go test -run XXX -bench . -benchmem ./internal/<pkg>/
go vet ./...
# profiling: add net/http/pprof only on an internal, guarded listener and only with approval
go tool pprof -http=:0 http://127.0.0.1:<pprof-port>/debug/pprof/heap
go tool pprof http://127.0.0.1:<pprof-port>/debug/pprof/goroutine
# MySQL (local compose)
docker compose -f docker-compose.dev.yml exec mysql mysql -uroot -p -e "SHOW ENGINE INNODB STATUS\G"
EXPLAIN ANALYZE <query>;
# load (local only), e.g. with hey/vegeta/k6 if installed
hey -z 30s -c 20 -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/me
```
Report `BLOCKED:` if a tool is unavailable; don't install new deps without approval.

## Output format
```text
Scope / hot paths:
Findings (severity, file:line, failure scenario under load):
Measurements (before/after, command, environment):
Fixes and their trade-offs:
Tests added (-race, duplicate execution, timeout):
Residual risks / capacity limits:
```
