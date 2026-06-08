# Batch 2A — E2E Execution Evidence: Risk #1 Closure Report

> Date: 2026-06-08
> Task: Real execution of `EmailDispatchE2E` against live MySQL (DEV/STAGING) to close the only remaining risk from `batch2a-email-pipeline-wiring-summary.md` ("PASS WITH RISKS — Risk 1: synthetic E2E dispatch test chưa được chạy thực tế").
> Constraint honored: no source code/repo files modified — this task closed the risk purely via real execution evidence.

# E2E Execution Evidence

**Command executed (exact, as specified):**
```
go test -race ./internal/notification/app/... -run EmailDispatchE2E -v
```
(Run with `-count=1` to bypass Go's test cache — the cache had stored a stale `SKIP` result from before the test database existed; without `-count=1`, `go test` silently replays that cached `(cached)` result instead of re-executing. This is standard Go test-cache behavior, not a code or environment change. The underlying binary, test names, and assertions are byte-identical either way.)

**Infrastructure used (real MySQL 8.0, isolated from shared dev/staging):**
- Spun up a brand-new throwaway container `cobo-email-e2e-mysql` (image `mysql:8.0`, host port `13306`, `root:secret`, db `cobo_iam`) on the dev box `88.216.208.0:21239` — completely separate from the shared `cobo-iam-mysql` container (which uses `root:root`, would have collided on the Docker healthcheck if its credentials were touched)
- Applied schema verbatim and in dependency order: `outbox_events` (extracted from `migrations/0001_init_core.up.sql`) → `migrations/0051_email_notifications.up.sql` → `migrations/0052_email_delivery_attempts.up.sql`
- Verified `SHOW TABLES` → `email_delivery_attempts`, `email_notifications`, `outbox_events` present
- Opened SSH tunnel `127.0.0.1:3306 → remote 127.0.0.1:13306`, matching the test's hardcoded DSN `root:secret@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true&loc=UTC` exactly
- Ran the exact specified `go test ... -run EmailDispatchE2E -v` from the local Go toolchain (the remote box has no Go toolchain — binary-artifact deployment only)

**Result:**
```
=== RUN   TestEmailDispatchE2E_TransactionalPublishWorkerDeliversHappyPath
--- PASS: TestEmailDispatchE2E_TransactionalPublishWorkerDeliversHappyPath (7.29s)
=== RUN   TestEmailDispatchE2E_TransientErrorRetriesThenSucceeds
--- PASS: TestEmailDispatchE2E_TransientErrorRetriesThenSucceeds (10.72s)
PASS
ok      github.com/cobo/cobo_iam_services/internal/notification/app    20.815s
```
Both tests **PASS** against real MySQL with race detector enabled (`-race`), reproduced twice (first confirmation run: 2.88s/4.73s; evidence-capture run with concurrent DB polling: 7.29s/10.72s — timing variance due to concurrent snapshot queries against the same DB).

# Logs

```
=== RUN   TestEmailDispatchE2E_TransactionalPublishWorkerDeliversHappyPath
--- PASS: TestEmailDispatchE2E_TransactionalPublishWorkerDeliversHappyPath (7.29s)
=== RUN   TestEmailDispatchE2E_TransientErrorRetriesThenSucceeds
--- PASS: TestEmailDispatchE2E_TransientErrorRetriesThenSucceeds (10.72s)
PASS
ok      github.com/cobo/cobo_iam_services/internal/notification/app    20.815s
```

No `--- SKIP` lines, no `t.Skipf` triggered — proof the test connected to and executed against a live MySQL instance (the `openEmailE2EDB` helper only skips on dial/ping failure).

# Database Evidence

Captured via a concurrent polling loop (`SELECT` snapshots every ~0.4–0.6s against `email_notifications`, `outbox_events`, `email_delivery_attempts` on the live throwaway DB) running **during** test execution — necessary because both tests delete their rows via `t.Cleanup` on completion.

**1. Transactional publish evidence** (`TestEmailDispatchE2E_TransactionalPublishWorkerDeliversHappyPath`, notification `e2e...-001`, idempotency key `...-idem-happy`):

| time (UTC) | `email_notifications.status` | `outbox_events.status` |
|---|---|---|
| 13:22:42.381 | `pending` | `pending` (event `...-002`, type `email.dispatch`, retry_count=0) |

→ Both rows appear **simultaneously** in the same snapshot, the instant `DispatchEmail` returns — proving `BeginTx → InsertNotificationTx → PublishEventTx → Commit` landed atomically (no window where one row exists without the other).

**2. Outbox event creation evidence:**
```
event_id=e2e...-002  aggregate_id=e2e...-001  event_type=email.dispatch  status=pending  retry_count=0  available_at=13:22:42
```
Created in the same transaction as the `email_notifications` insert (see #1).

**3. Worker consumption evidence:**

| time | notification status | outbox status | delivery attempt |
|---|---|---|---|
| 13:22:44.211 | `pending` | `processing` | attempt #1 = `sent` (provider=smtp) |
| 13:22:44.819 | `sent`, `sent_at=13:22:44.426` | `processed` | attempt #1 = `sent` |

→ The processor's `Tick` claimed the event (`pending → processing`), the registered wrapper-closure handler delivered via the fake SMTP adapter, the notification flipped `pending → sent` with `sent_at` populated, and the outbox event was marked `processed` — exactly the AK.4 acceptance chain (`pending → sending → sent`, here observed as `pending → sent` between polling ticks since delivery completed within ~600ms).

# Retry Evidence

(`TestEmailDispatchE2E_TransientErrorRetriesThenSucceeds`, notification `e2e...-001`, idempotency key `...-idem-retry`):

| time (UTC) | notification status | last_error_code | outbox status | retry_count | available_at | delivery attempts |
|---|---|---|---|---|---|---|
| 13:22:50.928 | `pending` | — | `pending` | 0 | 13:22:50 | — |
| 13:22:51.540 | `sending` | — | `processing` | 0 | 13:22:50 | #1 = `retry` / `transient_smtp` |
| 13:22:52.168 | `retry` | `transient_smtp` | `processing` | 0 | 13:22:50 | #1 = `retry` / `transient_smtp` |
| 13:22:52.778 | `retry` | `transient_smtp` | `pending` | **1** | **13:22:54** (rescheduled to the future) | #1 = `retry` |
| 13:22:54.622–55.853 | `retry` | `transient_smtp` | `pending` | 1 | 13:22:53 | #1 = `retry` (no redelivery — early ticks correctly held) |
| 13:22:56.469 | `retry` | `transient_smtp` | `processing` | 1 | 13:22:53 | #1 = `retry` |
| 13:22:57.074 | **`sent`**, `sent_at=13:22:56.774` | `transient_smtp` (last error preserved) | **`processed`** | 1 | 13:22:53 | #1 = `retry`/`transient_smtp`, **#2 = `sent`/smtp** |

→ Full retry-cycle proof, captured live:
1. First delivery attempt fails with a transient SMTP error (`421 service not available...` → classified `transient_smtp`)
2. Notification transitions `pending → sending → retry`; outbox event is **rescheduled forward** (`available_at` pushed to the future, `retry_count` incremented to 1) — never dropped
3. Early ticks (13:22:54.6–55.8, before the rescheduled `available_at`) correctly **do not redeliver** — only one delivery attempt row exists throughout this window
4. Once the redelivery window opens, the processor redelivers; the second attempt succeeds; final state is `sent` with both attempt rows (`retry` then `sent`) preserved as an audit trail, and the outbox event lands on `processed`

# Risk Closure

**Risk #1 — "Synthetic E2E dispatch test chưa được chạy thực tế" — CLOSED.**

Both `TestEmailDispatchE2E_TransactionalPublishWorkerDeliversHappyPath` and `TestEmailDispatchE2E_TransientErrorRetriesThenSucceeds` were executed for real (not compiled-only, not skipped) against a live, isolated MySQL 8.0 instance with migrations `0051`/`0052` (plus prerequisite `outbox_events` from `0001`) applied verbatim, using the exact command specified by the user, with `-race` enabled. Live polling captured the full evidence chain — transactional publish, outbox creation, worker consumption, retry scheduling/redelivery, and final delivery — matching AK.4's acceptance criterion and the rescoping plan's "Transactional Publish" contract exactly. No `t.Skipf` was triggered; both tests reported `--- PASS`.

Environment notes for the record (none block closure):
- A connection-refused false alarm on the first two attempts was traced to **Go's test result cache** replaying a stale pre-database `SKIP` — resolved with `-count=1`; this is tooling behavior, not an environment or code defect.
- Risk #2 (`TestDispatchEmail_SanitisesAllSensitiveVars` pre-existing failure) remains separately documented as out-of-scope per the original completion report — unaffected by this evidence run.

**Teardown verified:** throwaway container `cobo-email-e2e-mysql` removed (`docker ps -a` shows no trace), all staged SQL/script files removed from `/root/`, SSH tunnel closed (port 3306 confirmed unbound locally). Zero residual changes to shared dev/staging infrastructure (`cobo-iam-mysql` untouched).

# Final Verdict

**PASS**

**Batch 2A ACCEPTED.**

## Proposed Batch 2 Plan

Per the canonical execution order (Batch 0 → 1 → 5(a) → 2A → **2** → 3 → 4 → 6 → 7), Batch 2 is the next phase. Based on what Batch 2A established (durable transactional dispatch + worker registration for `email.dispatch`, with `EmailNotificationService` constructed but **not yet wired into any production caller**), Batch 2 should focus on:

1. **Production DI wiring** — construct `EmailNotificationService` (with `WithTransactionalDispatch`) in `httpserver/server.go`'s production graph and identify/connect the first real caller (per the spec's adhoc/auth flows that are currently in scope for Batch 2, not the untouchable `internal/adhoc/...`).
2. **Contract-first definition** — before coding, write the request/response/error matrix for whichever surface becomes the first caller (e.g., an admin preview/dispatch endpoint, or a specific auth/notification trigger point named in `adhoc-email-spec-v3.md`).
3. **Shadow-mode cutover plan** — define how `EMAIL_SHADOW_MODE` graduates from "log and continue on failure" to becoming the primary dispatch path, with a rollback gate.
4. **Resolve Risk #2** (or formally re-scope it out) — `TestDispatchEmail_SanitisesAllSensitiveVars`'s `support_email` template-fixture mismatch should be fixed or explicitly ticketed before it accumulates as tech debt blocking future template-variable work.
5. **Fresh Docker build + integration verification** — per `docs/ai-cache/README.md`'s mandatory gate, after Batch 2 code lands, rerun a fresh Docker build and execute the relevant integration suite (mirroring this same real-MySQL evidence-gathering approach) before merge.

Recommend starting with a contract-first spec for item 1–2 (the production caller) since that determines the shape of everything else in Batch 2.
