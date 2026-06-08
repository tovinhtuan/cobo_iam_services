# Batch 2A — Durable Email Pipeline Wiring: Completion Report

> Source of truth read in order: `adhoc-email-spec-v3.md` → Batch 2A Grounding Report → `adhoc-email-batch2a-rescoping-plan.md` → "Pre-Implementation Verification result: PASS — READY TO IMPLEMENT BATCH 2A"
> Date: 2026-06-08

## Pre-Implementation Verification

Per the rescoping plan's Step 1 gate, every constructor/interface signature was read before writing code:

- `NewEmailDispatchHandler(notifRepo, attemptRepo, registry, renderer, adapter, idg, clock, maxAttempts)` — matches
- `NewEmailNotificationRepository(db)`, `NewEmailDeliveryAttemptRepository(db)` — matches
- `WithTransactionalEnqueue` / `toOutboxEvent` / `deliverAuthEmailEvent` patterns (the proven mirrors) — matches
- `outboxmysql.Repository.PublishEventTx(ctx, tx, events.Event)` — matches the `OutboxPublisher` shape required
- `platformoutbox.Processor.Register(eventType, Handler)` / `HandlerFunc` / `QueuedEvent.PayloadJSON` — matches

**Result: zero drift.** Proceeded without redesign, per the mandate ("không tạo implementation plan mới, không review lại, không redesign, không mở rộng scope").

## Files Changed

```
 cmd/worker/main.go                                                  | 28 ++++++
 internal/notification/app/email_dispatch_contracts.go              | 20 +++++
 internal/notification/app/email_service.go                         | 95 ++++++++++++-
 internal/notification/infra/mysql/email_notification_repository.go | 20 ++-
 internal/notification/app/email_dispatch_e2e_test.go (new)         | 305 ++++++++++++
```

## Transactional Dispatch Implementation

Mandated pattern implemented exactly: **Option A — Transactional Publish**, `BeginTx → InsertNotificationTx → PublishEventTx → Commit`, no best-effort publish.

- `email_dispatch_contracts.go`: added `OutboxPublisher{ PublishEventTx(ctx, tx, event) }` and `TxEmailNotificationRepository{ EmailNotificationRepository + InsertNotificationTx(ctx, tx, n) }`
- `email_notification_repository.go`: `InsertNotificationTx` shares an `insertNotification(ctx, ex emailExecer, n)` helper with `InsertNotification` via an `emailExecer` (`*sql.DB`/`*sql.Tx`) seam — mirrors the sibling repository's `execer`/`createJob` pattern
- `email_service.go`: added `sqlDB`/`outbox` fields, `EmailServiceOption`, `WithTransactionalDispatch(db, outbox)`. `DispatchEmail` branches: when transactional opts are set **and** `repo` satisfies `TxEmailNotificationRepository`, calls `dispatchTransactional` (insert + publish + commit atomically, with `defer tx.Rollback()` and `ErrAlreadyDispatched` replay short-circuit); otherwise the existing non-transactional path runs unchanged — every existing construction site keeps compiling/passing
- `toEmailOutboxEvent` builds the `email.dispatch` envelope carrying the **plain** `req.Variables` (never `VariablesJSONSanitized`) — the worker handler needs the real values (e.g. OTP) to render, while the persisted row stores only the redacted copy

**Naming deviation (necessary, not a redesign):** the plan's proposed type name `ServiceOption` collides with the pre-existing `notification.service.ServiceOption` in the same `app` package (`func(*service)` vs `func(*EmailNotificationService)` — Go redeclaration error). Renamed to `EmailServiceOption`; everything else matches the plan verbatim.

## Worker Registration Implementation

Inside the existing `if sqlDB != nil { ... }` gate in `cmd/worker/main.go` (the handler needs real DB-backed repos), constructed `EmailNotificationRepository`, `EmailDeliveryAttemptRepository`, the SMTP `DeliveryAdapter` (mirroring `httpserver/server.go`'s `notificationsmtp.NewAdapter(Config{Host/Port/User/Pass/From: cfg.SMTP*}, nil)`), and `EmailDispatchHandler`, then registered via the **mandated wrapper closure** — never a direct method-value registration:

```go
processor.Register(notificationapp.EmailDispatchOutboxEventType, platformoutbox.HandlerFunc(func(ctx context.Context, event platformoutbox.QueuedEvent) error {
    return emailDispatchHandler.Handle(ctx, event.PayloadJSON)
}))
```

This mirrors `deliverAuthEmailEvent`'s wrapper-closure shape exactly (`event.PayloadJSON → handler.Handle(ctx, payload)`).

**Production DI decision (resolves the plan's open question — "is 'constructed and reachable, proven only by the synthetic test' sufficient?"):** YES. Did **not** construct `EmailNotificationService` in `httpserver/server.go`'s production DI graph. There is no in-scope caller — `adhoc`/Batch 2 are untouchable, and no preview/dispatch admin route exists. Constructing it with no caller would be dead code / scope creep into Batch 2 or new-surface territory — both forbidden. AK.4's acceptance criterion ("a synthetic `email.dispatch` event is published, consumed by the registered handler, and observed transitioning pending → sending → sent/failed_permanent") is satisfied by the new E2E test alone.

## Synthetic E2E Test Evidence

New file `internal/notification/app/email_dispatch_e2e_test.go` (package `app_test`), wiring the **genuine production chain** end-to-end against real MySQL — only the SMTP transport is faked:

`EmailNotificationService(WithTransactionalDispatch)` → real `outboxmysql.Repository` → real `platformoutbox.Processor.Tick` → the **exact wrapper-closure registered in `cmd/worker/main.go`** → `EmailDispatchHandler` → fake `DeliveryAdapter`

1. **`TestEmailDispatchE2E_TransactionalPublishWorkerDeliversHappyPath`**
   dispatch inserts+publishes atomically (status=pending, outbox row=pending) → one `Tick` delivers → status=sent, `sent_at` set, adapter called once with the real OTP in the body, outbox row=processed → replay with the same idempotency key returns the same row with no duplicate outbox publish and no resend on a follow-up tick (replay-safety)

2. **`TestEmailDispatchE2E_TransientErrorRetriesThenSucceeds`** (the required retry scenario)
   first attempt → transient SMTP error → notification=`retry` / `last_error_code=transient_smtp`, outbox row stays `pending` with a future `available_at` (redelivery scheduled, never dropped) → an early tick does **not** redeliver → forcing `available_at` into the past (simulating backoff elapsing) → processor redelivers → second attempt succeeds → status=`sent`, outbox row=`processed`

Follows the `internal/iam/infra/mysql/credentials_subscription_test.go` `openTestDB`/`t.Skipf` convention (`root:secret@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true&loc=UTC`), with a run-unique `e2eIDGen` ID prefix and `t.Cleanup` deletes (cascading via the `email_delivery_attempts` FK `ON DELETE CASCADE`) so reruns against a persistent DB never collide or accumulate rows.

## Test Results

```
go build ./...                                                           ✅
go vet ./...                                                             ✅
go test -race ./internal/notification/... ./internal/platform/outbox/...
```

- All packages **pass** except one pre-existing, unrelated failure: `TestDispatchEmail_SanitisesAllSensitiveVars` — fails identically on the unmodified baseline commit `90b3db5` (verified via `git stash -u` + rerun): a template/test-fixture mismatch where the `auth.user_invitation.new_user_company` template requires `support_email` but the test's `Variables` map omits it. **Not a Batch 2A regression**; out of scope to fix here ("không mở rộng scope").
- The two new E2E tests **compile and run cleanly**, hitting `t.Skipf` (`--- SKIP`) because no MySQL/Docker is available in this sandbox (`docker ps` → "no docker cmd"). They are written to run for real in staging (e.g. the dev box at `88.216.208.0:21239`).

**BLOCKED (environment): could not execute the synthetic E2E assertions against a live MySQL in this sandbox.** Recommend running:
```
go test -race ./internal/notification/app/... -run EmailDispatchE2E -v
```
in staging/CI (where `MYSQL_DSN`/Docker are available and migrations 0051/0052 are applied) before considering Batch 2A merge-ready.

## Rollback Notes

All changes are additive and behind explicit opt-in:
- `WithTransactionalDispatch` is an opt-in `EmailServiceOption` — no existing `NewEmailNotificationService` call site was changed; the legacy `InsertNotification` path is untouched and remains the default
- The `email.dispatch` worker registration is gated on `sqlDB != nil` (same gate as the rest of the email-capable block in `cmd/worker/main.go`) — reverting is a clean revert of the added `if` block plus two imports
- No migrations, no schema changes, and no changes to `internal/adhoc/...`, `AdhocProposalNotifier`, `internal/reminder/...`, retry/backoff constants, or `EMAIL_SHADOW_MODE`
- To roll back: revert the 4 modified files and delete the new test file — no data migration required

## Final Verdict

**PASS WITH RISKS**

- Risk 1 (environment-only): the synthetic E2E test could not be executed against live MySQL in this sandbox; it is written and compiles correctly but needs a staging/CI run to produce *executed* evidence (not just compiled evidence) before merge.
- Risk 2 (pre-existing, documented, out of scope): `TestDispatchEmail_SanitisesAllSensitiveVars` fails on baseline and on this branch identically — a template-fixture issue unrelated to Batch 2A.

Detailed task-update entry also recorded in `docs/ai-cache/reusable-task-updates.md` under "2026-06-08 - Batch 2A: wire durable email pipeline end-to-end (Transactional Publish + worker registration)".
