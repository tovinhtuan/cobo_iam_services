# Batch 2 — Phase 0 Pre-Implementation Verification: DRIFT REPORT (STOP)

> Date: 2026-06-08
> Persona: Principal Backend Engineer + Principal Messaging Engineer + Principal SRE
> Trigger: mandatory Phase 0 seam verification before implementing the approved `adhoc-email-batch2-implementation-plan.md` (READY FOR IMPLEMENTATION)
> Outcome: **STOP** — none of the plan's anticipated seam options (A/B/C) is implementable without crossing an explicit STOP boundary (new migration / `internal/notification` architecture change / worker-registration-contract change). No implementation code was written. No files were modified.

---

## Pre-Implementation Verification

**Chosen Seam: NONE — all three (A/B/C) require crossing a STOP condition**

### Findings (file:line, exact signatures, all read live from HEAD)

1. **`EmailDispatchHandler` constructor** — `internal/notification/app/email_dispatch_handler.go:48-56`:
   ```go
   func NewEmailDispatchHandler(notifRepo EmailNotificationRepository, attemptRepo EmailDeliveryAttemptRepository,
       registry TemplateRegistry, renderer EmailRenderer, adapter DeliveryAdapter,
       idg idgen.Generator, clock func() time.Time, maxAttempts int) *EmailDispatchHandler
   ```
   Takes exactly **one concrete `DeliveryAdapter`** at construction time — baked into the struct (`h.adapter`); no per-call selection mechanism exists.

2. **Worker registration** — `cmd/worker/main.go:104-117`: constructs **exactly one** `emailDispatchHandler` (wired to the real SMTP adapter) and registers it once: `processor.Register(notificationapp.EmailDispatchOutboxEventType, HandlerFunc(...))`. The processor (`internal/platform/outbox/processor.go:20-38`) stores handlers in `handlers map[string]Handler`; `Register` does a flat `p.handlers[eventType] = h` — **last registration silently overwrites the prior one**. No multi-handler-per-event-type support exists anywhere in the platform outbox processor.

3. **`DeliveryAdapter` injection point** — single concrete value, injected once at handler-construction time. `DeliveryMessage` (`internal/notification/app/email_delivery.go:21-28`) — the only thing passed to `Send` — carries **zero routing signal**: `{NotificationID, To, Subject, TextBody, HTMLBody}`. No `SourceAggregateType`, no `CompanyID`, no shadow marker of any kind.

4. **`EmailNotificationService` construction** — `internal/notification/app/email_service.go:47-66`: `NewEmailNotificationService(repo, registry, renderer, idg, clock, opts ...EmailServiceOption)`; `WithTransactionalDispatch(db, outbox)` only sets `sqlDB`/`outbox`. Critically, `toEmailOutboxEvent` (`email_service.go:196-208`) **hardcodes** `EventType: EmailDispatchOutboxEventType` — a **package-level constant** (`= "email.dispatch"`, `email_dispatch_handler.go:17-18`) whose own doc-comment states it is intentionally locked "so the publisher and consumer cannot drift apart." There is **no `EmailServiceOption`** to override the published event type.

5. **SMTP adapter construction** — `cmd/worker/main.go:97-103`: `notificationsmtp.NewAdapter(notificationsmtp.Config{Host: cfg.SMTPHost, ...}, nil)`, constructed once, injected once into the single handler.

### Why none of A/B/C is achievable

- **A — Per-event adapter routing**: requires a signal reachable at `Send`-time distinguishing "terminate in real SMTP" vs. "terminate in recorder." No such signal exists in `DeliveryMessage`. The only persisted candidate, `email_notifications.source_aggregate_type = 'ad_hoc_proposal'`, **cannot distinguish** a Branch-A (post-cutover authoritative durable) dispatch from a Branch-B (shadow durable) dispatch — both would carry the identical `source_aggregate_type`/`SourceEventType`. Manufacturing a distinguishing signal requires either extending `DeliveryMessage`/`EmailNotification` (→ changes `internal/notification` architecture) or persisting a new shadow-marker column (→ new migration). **Both are explicit STOP conditions.**

- **B — Dual handler registration**: requires two distinct outbox `event_type` strings so a second `processor.Register(...)` call doesn't silently overwrite the first (the map is keyed by event type, last-write-wins). But `EmailDispatchOutboxEventType` is a hardcoded constant baked unconditionally into every `DispatchEmail` call's published envelope, with **no override hook** — making it overridable is a direct edit to the publisher contract that the existing code explicitly locks down via its own doc-comment. → "cần sửa internal/notification architecture" / "cần sửa worker registration contract".

- **C — Dual service construction**: collapses into the identical problem as B one layer up. Two `EmailNotificationService` instances still publish through the same hardcoded `"email.dispatch"` event type into the same single registered handler/adapter — constructing two services changes nothing about which handler ultimately receives the event. This is also the exact pattern the approved plan itself explicitly forbids ("Không fork EmailDispatchHandler. Không duplicate pipeline").

### Conclusion

The approved plan's Step 7 correctly anticipated this as the one open seam question and assumed it would resolve cleanly into A, B, or C. On actual inspection of HEAD, **the single hardcoded `EmailDispatchOutboxEventType` constant + single `processor.handlers` registration (last-write-wins, no multi-handler support) + single concrete `DeliveryAdapter` + zero per-message routing data together make all three options structurally require either a new migration, an `internal/notification` architecture change, or a worker-registration-contract change** — i.e., every viable path crosses a boundary explicitly marked as a STOP condition for this task.

This is **not** something resolvable by picking the "closest" option without silently violating a forbidden boundary, and not something to redesign around per the task's constraints ("Không tự redesign").

---

## Files Changed
*(none — implementation halted at the Phase 0 gate; zero source files modified)*

## Config Changes / RecordingDeliveryAdapter / Notifier Migration / CF-12 Closure / DI Wiring / Metrics / Tests Added / Test Results
*(not started — Phase 0 did not produce a valid seam to build on; proceeding into any of these phases would require crossing an explicit STOP condition)*

## Risks
- The plan's D1/D2/D3 architectural decisions (Revised D1 = legacy real + durable real-pipeline-with-shadow-adapter; D2 = `notifier.go` gating; D3 = `ADHOC_EMAIL_OUTBOX_ENABLED` keys) **remain sound and unaffected** — the drift is purely at the seam-mechanics level (how to route one of two terminal adapters through a pipeline that was correctly built, for Batch 2A's narrower scope, assuming exactly one terminal adapter per registered event type).
- This drift was not visible at plan-review time because it only surfaces when reading the outbox-registration code and the `EmailDispatchOutboxEventType` constant's publisher-side usage together — exactly the live-code verification Phase 0 exists to perform, and exactly what the plan's Step 7 flagged as the one item requiring it before implementation could safely begin.

## Rollback Verification
*(N/A — no code changed, nothing to roll back)*

## Final Verdict

# **FAIL**

Phase 0 pre-implementation verification did not lock a valid seam. All three plan-anticipated options (A — per-event adapter routing, B — dual handler registration, C — dual service construction) require crossing an explicit STOP condition (new migration, `internal/notification` architecture change, or worker-registration-contract change). Per the task's STOP conditions, implementation was halted here, the drift was reported, and no redesign or workaround was attempted.

### Recommended next step (an architectural decision — outside this report's authority to make)

The Canonical Decision Record / D1 Reassessment needs **one additional, narrowly-scoped architectural decision**: *how the durable pipeline is permitted to terminate in two different adapters for the same outbox event type*. Plausible directions for the architecture owner to choose between (not prescribed here):
- (a) a small, additive, backward-compatible extension making the published outbox `event_type` configurable via a new `EmailServiceOption` (touches `internal/notification/app/email_service.go` + the worker registration — requires explicit sign-off from whoever owns the "publisher/consumer cannot drift apart" invariant on `EmailDispatchOutboxEventType`); or
- (b) extending `DeliveryMessage`/`EmailNotification` with a routing/shadow-marker field reachable at `Send`-time (touches `internal/notification` contracts, and depending on persistence needs may require a new migration).

Either path requires authority this task explicitly does not grant ("KHÔNG redesign", "STOP nếu cần sửa internal/notification architecture / worker registration contract / migration mới"). Batch 2 implementation should remain blocked until that decision is made and a revised/superseding plan reflects it.
