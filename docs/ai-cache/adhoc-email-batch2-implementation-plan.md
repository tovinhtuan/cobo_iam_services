# Batch 2 Final Implementation Plan

> Date: 2026-06-08
> Persona: Principal Software Architect + Principal Backend Engineer + Principal Messaging Architect + Principal SRE
> Authority order honored: **D1 Reassessment > Canonical Decision Record > Grounding Report > Spec Text** (D1 Reassessment wins on any conflict)
> Source of Truth read in order: `adhoc-email-spec-v3.md` → `adhoc-email-batch2-grounding-report.md` → `adhoc-email-batch2-decision-record.md` (superseded D1 marked) → `adhoc-email-batch2-d1-reassessment.md` (authoritative D1)
> Constraint honored: this document is a PLAN ONLY — no code, no diff, no Batch 2A re-review, no pipeline redesign

---

## Executive Summary

Batch 2 migrates the **caller** — `AdhocProposalNotifier` (4 `Notify*` methods) — from the legacy fire-and-forget `DeliveryAdapter.Send` path onto the durable `EmailNotificationService.DispatchEmail` pipeline that Batch 2A already proved end-to-end (`DispatchEmail → InsertNotificationTx → PublishEventTx → Commit → Outbox → Worker → EmailDispatchHandler → DeliveryAdapter`, **PASS**, **ACCEPTED**). Batch 2 does **not** build, alter, or re-prove that pipeline.

Per the authoritative **Revised D1** (D1 Reassessment, which supersedes the original Option A in the Canonical Decision Record): during the Shadow Mode validation window, the **legacy path keeps sending real emails** (remains system of record, unchanged), while the **durable path runs its complete genuine pipeline but terminates in a `RecordingDeliveryAdapter`** (a recording/shadow adapter mirroring the `fakeAdapter` class Batch 2A already built and got **ACCEPTED** for an equal-or-higher bar) instead of live SMTP. This reproduces `cobo_adhoc_email_shadow_total{outcome="match"|"mismatch"}` **byte-for-byte identically** to the spec's own operational definition (§AK.5 line 657: `COUNT(*) GROUP BY idempotency_key HAVING COUNT(*) > 1` on `email_notifications`) — because that signal is generated entirely upstream of the delivery-transport step — while sending **zero duplicate emails to real users**.

Two new config flags gate the rollout: `EMAIL_SHADOW_MODE` (existing, currently dormant — Batch 2 gives it its first runtime meaning) and `ADHOC_EMAIL_OUTBOX_ENABLED` (new, confirmed absent from `config.go` today). Cutover sequence: Shadow window clean (zero mismatches, 24h STAGING + 48h PROD) → flip `ADHOC_EMAIL_OUTBOX_ENABLED=true` → durable path becomes sole/authoritative sender → `EMAIL_SHADOW_MODE=false`. Rollback at any point is a single flag flip with the legacy path kept fully intact.

---

## Scope

### In Scope (Batch 2)

1. **Caller migration** — `internal/adhoc/infra/notification/notifier.go`: replace `n.delivery.Send(...)` in `sendEmail` with a routed call into `NotificationService.DispatchEmail(...)`, constructing `DispatchEmailRequest` with `IdempotencyKey: "adhoc.<event_type>.<proposalID>.<recipientMembershipID>"`. No wrapper of any kind (per §AK.5, verbatim).
2. **CF-12 closure** — `internal/adhoc/app/service.go`: delete `dispatchNotificationAsync`'s `context.Background()` goroutine spawn; pass `ctx` through directly to the 4 call sites (`SubmitProposal`, `FocalApprove`, `AdminApprove`, `Reject`).
3. **Two config flags** — `internal/platform/config/config.go`: give `EMAIL_SHADOW_MODE` (existing, dormant) its first runtime meaning for the adhoc module; register new `ADHOC_EMAIL_OUTBOX_ENABLED` (`AdhocEmailOutboxEnabled bool`, default `false`).
4. **`RecordingDeliveryAdapter`** — a new, narrowly-scoped adapter (implements `notifapp.DeliveryAdapter`) used **only** as the durable path's terminal adapter during Shadow Mode; mirrors Batch 2A's accepted `fakeAdapter` pattern.
5. **Production DI wiring** — `internal/httpserver/server.go`: construct `EmailNotificationService` (with `WithTransactionalDispatch`) in the adhoc module's existing `if cfg.WorkflowAdhocEnabled` block, and pass it + the two new flags into `adhocnotif.New(...)`.
6. **`cobo_adhoc_email_shadow_total{outcome, company_id}` emission** — computed exactly per §AK.5 line 657's operational definition (idempotency-key collision count on `email_notifications`), using the metric already named in §AH.1 row 5.
7. **Shadow-window verification + cutover gate** — operational runbook for the 24h STAGING + 48h PROD zero-mismatch windows and the `ADHOC_EMAIL_OUTBOX_ENABLED` flip.

### Out of Scope (explicitly NOT touched)

| Area | Reason | Belongs to |
|---|---|---|
| `internal/notification/app/email_service.go`, `email_dispatch_handler.go`, `email_dispatch_contracts.go`, `internal/notification/infra/mysql/...` | Batch 2A already wired and proved this end-to-end (PASS/ACCEPTED) — re-touching is re-design, forbidden | Batch 2A (closed) |
| Migrations `0051`/`0052` or any new migration | §AK.5 "Must-Update-Migration: none"; Success Criteria forbid new migrations | — |
| `internal/adhoc/observability/...`, any new metrics beyond `cobo_adhoc_email_shadow_total` | Batch 5(a) territory (instrumentation already shipped) | Batch 5(a) (closed) |
| Email template content/structure for `adhoc.*` keys | Content/template work | Batch 3 |
| Audit-trail schema or audit-write semantics for adhoc emails | Audit/compliance work | Batch 4 |
| Prometheus recording rules (`cobo_adhoc_email_pipeline_match_ratio`), alert rules, reconciliation jobs | §AH.1 row 11 explicitly assigns these to Batch 6 | Batch 6 |
| Historical data repair (DF-02/DF-03/DF-05), `email_notifications.status` ENUM extension, `superseded_by` column, migration `0093` | §AF.1/§AK.9 — repair execution is Batch 7 | Batch 7 |
| `internal/iam/...` (`publishEmail`/`deliverAuthEmailEvent`), `internal/reminder/...` (`EmailSender`) | Different pipelines, outside §AK.5's `internal/adhoc/...` scoping | — |
| Retry/backoff constants (`MaxEmailDeliveryAttempts`, `EmailRetryBackoff`) | Already finalized by ADR-3, zero changes needed | — |

---

## Architecture

### Before (current state — confirmed via grounding)

```
service.go (4 call sites)
  → dispatchNotificationAsync(fn)
      → go func() { fn(context.Background()) }()      ← CF-12: severs request-scoped ctx
          → AdhocProposalNotifier.Notify*
              → sendEmail(ctx, to, templateKey, vars, proposalID, label)
                  → n.delivery.Send(ctx, DeliveryMessage{
                        NotificationID: "adhoc.<label>.<proposalID>",   ← no idempotency guarantee (CF-03/04)
                        To, Subject, TextBody, HTMLBody })
                      → real SMTP, synchronous, errors only n.log.Warn — no retry/outbox (CF-14)
```

### After (target state — Batch 2 end-state, post full cutover)

```
service.go (4 call sites)
  → AdhocProposalNotifier.Notify*  (ctx threaded directly — no goroutine, CF-12 closed)
      → sendEmail(ctx, to, templateKey, vars, proposalID, recipientMembershipID, eventType)
          → NotificationService.DispatchEmail(ctx, DispatchEmailRequest{
                To, TemplateKey, Locale: "vi", Variables,
                IdempotencyKey: "adhoc.<event_type>.<proposalID>.<recipientMembershipID>",
                TriggeredByUserID, SourceEventType, SourceAggregateType: "ad_hoc_proposal",
                SourceAggregateID: proposalID, CompanyID })
              → BeginTx → InsertNotificationTx → PublishEventTx → Commit   (Batch 2A — proven, untouched)
                  → outbox (email.dispatch) → Worker → EmailDispatchHandler → DeliveryAdapter (real SMTP)
```

### Transition state (Shadow Mode window — the actual Batch 2 runtime behavior being designed here)

```
sendEmail(ctx, ...)
  ├─ if cfg.AdhocEmailOutboxEnabled:                       // post-cutover: durable is sole/authoritative
  │     → DispatchEmail(... real DeliveryAdapter at the tail ...)   [legacy branch not entered]
  │
  ├─ else if cfg.EmailShadowMode:                          // Shadow window: legacy authoritative, durable shadowed
  │     → n.delivery.Send(...)                              [REAL send — system of record, unchanged]
  │     → (best-effort, error-isolated) DispatchEmail(... RecordingDeliveryAdapter at the tail ...)
  │           [shadow run — produces the email_notifications row + idempotency-key signal,
  │            never reaches a real mailbox; failures here NEVER affect the legacy send above]
  │
  └─ else:                                                  // pre-migration default (current production behavior)
        → n.delivery.Send(...)                              [unchanged — identical to today]
```

This three-way branch is the complete behavioral surface Batch 2 introduces. It lives entirely inside `sendEmail` — no new wrapper layer, consistent with §AK.5's "no wrapper of any kind is introduced or required" and with config.go's own existing comment on `EmailShadowMode` (line 104-105: *"mirrors dispatch into the new pipeline without taking over delivery... Shadow failures must never break the legacy path"*) — which is the spec's own pre-existing design intent for this exact flag, now finally given a concrete implementation.

---

## Shadow Mode Design

### Flag semantics

| Flag | Type | Default | Meaning |
|---|---|---|---|
| `EMAIL_SHADOW_MODE` → `cfg.EmailShadowMode` | `bool` | `false` (existing, `config.go:106/191`) | When `true` (and `ADHOC_EMAIL_OUTBOX_ENABLED=false`): mirror every adhoc dispatch into the durable pipeline (terminating in `RecordingDeliveryAdapter`) alongside the real legacy send, for comparison purposes only |
| `ADHOC_EMAIL_OUTBOX_ENABLED` → `cfg.AdhocEmailOutboxEnabled` | `bool` | `false` (NEW) | When `true`: durable pipeline (real `DeliveryAdapter`) becomes the sole, authoritative sender; legacy path is no longer invoked |

### Behavior matrix

| `EMAIL_SHADOW_MODE` | `ADHOC_EMAIL_OUTBOX_ENABLED` | Authoritative sender | Durable sender runs? | Shadow sender runs? | Expected Behavior |
|---|---|---|---|---|---|
| `false` | `false` | **Legacy** (`DeliveryAdapter`, real SMTP) | No | No | **Default / pre-migration state.** Identical to today's production behavior — `sendEmail` calls `n.delivery.Send` directly, nothing else. This is the Batch 2 *deploy-time* default in all environments before the Shadow window opens. |
| `true` | `false` | **Legacy** (`DeliveryAdapter`, real SMTP) | No (only shadow-mode invocation, not authoritative) | **Yes** — durable pipeline runs to completion terminating in `RecordingDeliveryAdapter` | **Shadow window (the validation state).** Legacy sends for real and remains system of record. Durable path runs the complete genuine pipeline (transactional publish → outbox → worker → handler → template render → retry/backoff → status tracking) but never reaches a live mailbox. `cobo_adhoc_email_shadow_total{outcome}` is emitted from the resulting `email_notifications` rows. Shadow-side errors are caught and logged — they must never propagate to or block the legacy send (per config.go's own pre-existing design comment for `EmailShadowMode`). |
| `false` | `true` | **Durable** (`DeliveryAdapter`, real SMTP, via `EmailNotificationService`) | **Yes — authoritative, real delivery** | No | **Post-cutover (steady state).** The durable pipeline is now the sole sender with a real terminal `DeliveryAdapter`; legacy is no longer invoked. This is the end-state Batch 2 is migrating toward. Reached only after the Shadow window has exited clean (zero mismatches, full 24h STAGING + 48h PROD) per §AE.3's gate. |
| `true` | `true` | **Durable** (`DeliveryAdapter`, real SMTP) | **Yes — authoritative, real delivery** | N/A — `ADHOC_EMAIL_OUTBOX_ENABLED` takes precedence; shadow branch is not entered | **Transient/operational state**, not a steady state to design around. `ADHOC_EMAIL_OUTBOX_ENABLED=true` means cutover has already happened — running shadow comparison against an already-authoritative durable path is meaningless (there is no second "legacy" outcome left to compare against once legacy is no longer invoked). The implementation must treat `ADHOC_EMAIL_OUTBOX_ENABLED=true` as taking strict precedence: enter the "durable-authoritative" branch and skip the shadow branch entirely, regardless of `EMAIL_SHADOW_MODE`'s value. (Spec basis: §AE.2 "Re-enable [`EMAIL_SHADOW_MODE`] at any point during Batch 2A/Batch 2 to re-run the side-by-side comparison if a mismatch is suspected" — this presupposes legacy is still authoritative when shadow runs; it does not describe running shadow comparison *after* cutover.) |

### Authoritative / durable / shadow sender — definitions used consistently above

- **Authoritative sender**: the path whose email actually reaches the real recipient's mailbox and is the system of record for "was this notification delivered." Exactly one path is authoritative at any time — determined solely by `ADHOC_EMAIL_OUTBOX_ENABLED` (true → durable; false → legacy).
- **Durable sender**: `EmailNotificationService.DispatchEmail` invoked with the **real** `DeliveryAdapter` at the tail of `EmailDispatchHandler` — i.e., the durable pipeline operating in its normal, production, message-actually-sent mode. Active only when `ADHOC_EMAIL_OUTBOX_ENABLED=true`.
- **Shadow sender**: `EmailNotificationService.DispatchEmail` invoked with `RecordingDeliveryAdapter` at the tail — i.e., the durable pipeline running for real (transactional publish, outbox, worker, retry, status tracking all genuine) but never transmitting to a live mailbox. Active only when `EMAIL_SHADOW_MODE=true AND ADHOC_EMAIL_OUTBOX_ENABLED=false`. This is the row/column the entire D1 Reassessment is about — it is what makes "zero duplicate emails to real users" achievable while still producing the exact gating metric.

---

## Shadow Adapter Design

### Name: `RecordingDeliveryAdapter`

(Chosen to mirror existing codebase naming conventions — `notificationsmtp.NewAdapter`, the test-only `fakeAdapter` in `email_dispatch_e2e_test.go`, and the `notifapp.DeliveryAdapter` interface it must satisfy: `Send(ctx context.Context, msg DeliveryMessage) (DeliveryResult, error)`.)

**Responsibilities**
- Implement `notifapp.DeliveryAdapter` exactly (single method `Send`) so it is a drop-in substitute for the real SMTP adapter at the tail of `EmailDispatchHandler` — requiring zero changes to the handler, the worker registration, or any Batch 2A code.
- On every `Send` call: capture the fully-resolved `DeliveryMessage` (the same struct the real adapter would receive — `NotificationID`, `To`, `Subject`, `TextBody`, `HTMLBody`, i.e. the message **after** template resolution and rendering have already happened upstream in `EmailDispatchHandler`), and return a synthetic `DeliveryResult{ProviderMessageID: <generated>, Provider: "shadow"}` with a `nil` error — i.e., always report success, deterministically, with no dependency on live network/provider state.
- **Never** open a network connection, never call `smtp.SendMail` or any transport, never reach a real mailbox.

**Captured Data**

For each `Send` invocation, record (in-process, structured log — see Persistence Strategy):
- `notification_id` (= `NotificationID`, which is the durable pipeline's `email_notification_id` — already persisted by `InsertNotificationTx`, joinable to `idempotency_key`/`recipient_email`/`status` via the existing `email_notifications` table)
- `to`, `subject` (recorded for human-debuggability of the shadow run; the actual comparison signal does not need these — see Metric Design)
- a monotonic sequence/timestamp, to support "was this `Send` called more than once for the same `notification_id`" diagnostics during shadow-window investigation

**Persistence Strategy**

- **No new table, no new column, no new migration** (per Success Criteria and §AK.5 "Must-Update-Migration: none"). The adapter's `Send` calls are recorded via **structured `slog` logging only** (e.g., `log.Info("shadow delivery recorded", slog.String("notification_id", ...), slog.String("to", ...), slog.String("subject", ...))`).
- The durable, queryable system of record for the comparison is **already** `email_notifications` + `email_delivery_attempts` — both populated by the genuine `InsertNotificationTx`/`EmailDispatchHandler` machinery regardless of which adapter sits at the tail. The adapter itself needs to persist nothing beyond its log line; the metric (see below) is computed from those existing tables, not from adapter-captured state. This keeps the adapter a pure, stateless, swap-in shim — exactly mirroring how Batch 2A's `fakeAdapter` needed no persistence of its own (it asserted against `f.notifRepo`/`f.db`, the real repositories).

**Lifecycle**

- **Construction**: built once in `internal/httpserver/server.go`'s adhoc-module DI block, alongside the new `EmailNotificationService` construction — passed into `AdhocProposalNotifier` (or into the `EmailNotificationService` wiring used by the shadow branch) only when needed.
- **Activation**: selected at the `sendEmail` routing layer purely by config-flag state (see Shadow Mode Design matrix) — the adapter itself carries no flag-awareness; it is simply *which adapter the shadow-branch's `EmailNotificationService` instance was constructed with*.
- **Retirement**: deleted entirely once the Shadow window has exited clean and `EMAIL_SHADOW_MODE` is permanently turned off (§AE.2: "Turned off at the end of Batch 2's shadow window... remains available as a reusable platform-wide shadow-comparison primitive for future migrations" — i.e., the *flag* persists for future reuse, but this specific adapter instance and its wiring in the adhoc module's DI block can be removed once Batch 2's window closes; if a future migration needs shadow comparison again, it would construct its own `RecordingDeliveryAdapter` instance for its own pipeline).
- **Failure isolation**: the adapter itself cannot fail in a way that escapes (`Send` always returns success deterministically) — but the *caller* of the shadow-side `DispatchEmail` (in `sendEmail`) must still wrap that call so that any error from `DispatchEmail` itself (validation, DB, outbox publish) is caught and logged, never propagated to break the legacy send. This directly implements config.go's own pre-existing design comment for `EmailShadowMode` ("Shadow failures must never break the legacy path").

---

## File Map

### Must Change
| File | Change |
|---|---|
| `internal/adhoc/infra/notification/notifier.go` | Add `notificationService *notifapp.EmailNotificationService`, `shadowMode`, `outboxEnabled` (or equivalent flag-derived fields) to `AdhocProposalNotifier` struct + `New(...)` constructor params; rewrite `sendEmail` to the 3-way routing branch (Shadow Mode Design matrix); construct `DispatchEmailRequest` with the `adhoc.<event_type>.<proposalID>.<recipientMembershipID>` idempotency key; thread `recipientMembershipID`/`eventType` through the 4 `Notify*` call sites into `sendEmail` |
| `internal/adhoc/app/service.go` | Delete `dispatchNotificationAsync`'s `context.Background()` goroutine spawn (current L41-55); call `n.notifier.Notify*` directly with the live request `ctx` at all 4 sites (current L168, L212, L320, L373) — closes CF-12 |
| `internal/platform/config/config.go` | Add `AdhocEmailOutboxEnabled bool` field (near `EmailShadowMode bool` at L106) + `AdhocEmailOutboxEnabled: boolEnv("ADHOC_EMAIL_OUTBOX_ENABLED", false)` registration (near L191) |
| `internal/httpserver/server.go` | Inside the existing `if cfg.WorkflowAdhocEnabled { ... }` block (L435-468): construct `EmailNotificationService` with `WithTransactionalDispatch(pool, outboxRepo)`, construct `RecordingDeliveryAdapter`, and pass the service + both adapters (real `smtpDelivery` + new `RecordingDeliveryAdapter`) + `cfg.EmailShadowMode` + `cfg.AdhocEmailOutboxEnabled` into the (extended) `adhocnotif.New(...)` constructor, replacing the current `adhocnotif.New(inAppSvc, smtpDelivery, emailTemplateRegistry, emailRenderer, cfg.PublicWebBaseURL, log)` call |

### Must Create
| File | Purpose |
|---|---|
| `internal/adhoc/infra/notification/recording_adapter.go` (or equivalent path inside the adhoc notification infra package — naming/location to mirror `notifier.go`'s package `notification`) | `RecordingDeliveryAdapter` — implements `notifapp.DeliveryAdapter`; see Shadow Adapter Design |

No migration files. No new tables/columns. No new top-level packages.

### Must Not Change
- `internal/notification/app/email_service.go`, `email_dispatch_handler.go`, `email_dispatch_contracts.go`, `internal/notification/infra/mysql/email_notification_repository.go`, `email_delivery_attempt_repository.go`, `internal/notification/infra/registry/...`, `internal/notification/infra/smtp/...` — Batch 2A territory, proven and ACCEPTED, zero changes
- `cmd/worker/main.go`'s `email.dispatch` registration block (L93-117) — already correct per Batch 2A
- `migrations/0051_email_notifications.up/down.sql`, `migrations/0052_email_delivery_attempts.up/down.sql` — untouched, no new migration
- `internal/adhoc/observability/...` — Batch 5(a) territory
- `internal/adhoc/app/contracts.go` — `ProposalNotifier`/`MemberInfo`/`ProposalDTO` interfaces and structs are sufficient as-is (`MemberInfo.MembershipID` already exists for idempotency-key construction; `ProposalDTO.CompanyID`/`.ProposalID`/`.ChangeNote`/etc. already cover the variable-building needs)
- `internal/iam/...`, `internal/reminder/...` — different pipelines, out of §AK.5 scope
- `EmailRetryBackoff`, `MaxEmailDeliveryAttempts` constants — finalized by ADR-3, untouched

---

## Implementation Steps

> Ordered for a single implementer (per spec staffing recommendation: "one senior backend engineer... as the primary implementer for Batches 0/1/2A/2"). Each step is independently buildable/testable; later steps depend only on earlier ones in this list.

**Step 1 — Register `ADHOC_EMAIL_OUTBOX_ENABLED` in config**
Add `AdhocEmailOutboxEnabled bool` to the `Config` struct (adjacent to `EmailShadowMode bool`, `config.go:106`) and `AdhocEmailOutboxEnabled: boolEnv("ADHOC_EMAIL_OUTBOX_ENABLED", false)` to the loader (adjacent to `EmailShadowMode: boolEnv("EMAIL_SHADOW_MODE", false)`, `config.go:191`). Mirror the existing field's doc-comment style; state explicitly in the new comment that this flag, once `true`, makes the durable pipeline the sole authoritative sender for adhoc emails (see Shadow Mode Design matrix row 3). Update `configs/config.example.env` with the new key, default `false`. Run `go build ./...` and the existing `config_test.go` to confirm zero regressions (mirrors how `EmailShadowMode` is asserted at `config_test.go:72-73`; add an analogous default-value assertion for the new flag).

**Step 2 — Build `RecordingDeliveryAdapter`**
Create the new file inside `internal/adhoc/infra/notification/` (package `notification`, alongside `notifier.go`). Implement `Send(ctx context.Context, msg notifapp.DeliveryMessage) (notifapp.DeliveryResult, error)`: log the captured fields via structured `slog` at `Info` level, generate a synthetic `ProviderMessageID` (e.g., `"shadow-" + msg.NotificationID`), return `DeliveryResult{ProviderMessageID: ..., Provider: "shadow"}, nil`. No constructor dependencies beyond an optional `*slog.Logger` (mirror `New`'s `if log == nil { log = slog.Default() }` pattern from `notifier.go`). This is a small, pure, stateless type — write its unit test in the same step (TC-Shadow groundwork; see Test Plan).

**Step 3 — Extend `AdhocProposalNotifier` to hold the durable-pipeline dependencies and flags**
In `notifier.go`: add fields to the struct — `notificationService *notifapp.EmailNotificationService`, `recordingAdapter notifapp.DeliveryAdapter` (or pass it pre-wrapped inside a second `*notifapp.EmailNotificationService` instance constructed with the recording adapter — see Step 5's note on construction approach), `shadowMode bool`, `outboxEnabled bool`. Extend `New(...)`'s parameter list accordingly (additive — keeps the function name `New`, no wrapper type introduced). Update the doc-comment at the top of `notifier.go` (currently: "Emails are rendered from templates and dispatched directly via DeliveryAdapter (SMTP)") to describe the new three-way routed behavior.

**Step 4 — Rewrite `sendEmail` as the 3-way router**
Replace the body of `sendEmail` (current L129-159) with the routing logic from the "Transition state" diagram above:
- Compute `idempotencyKey := fmt.Sprintf("adhoc.%s.%s.%s", eventType, proposalID, recipientMembershipID)` (the format §AK.5 mandates verbatim; `eventType` values are the four template-key suffixes already in use — `focal_review_requested`, `controller_review_requested`, `proposal_approved`, `proposal_rejected` — each ≤27 chars, matching L143's stated maximum).
- Branch 1 (`outboxEnabled == true`): call `n.notificationService.DispatchEmail(ctx, DispatchEmailRequest{To: to, TemplateKey: templateKey, Locale: "vi", Variables: vars, IdempotencyKey: idempotencyKey, TriggeredByUserID: <actor — see note below>, SourceEventType: "adhoc." + eventType, SourceAggregateType: "ad_hoc_proposal", SourceAggregateID: proposalID, CompanyID: companyID})` using the **real-adapter-backed** service instance; log (`Warn`) on error, do not propagate (matches `ProposalNotifier`'s "fire-and-forget, must never propagate errors" contract, `contracts.go:95`).
- Branch 2 (`shadowMode == true && outboxEnabled == false`): (a) perform the **existing** `n.delivery.Send(...)` call exactly as today (real send, untouched code path — copy verbatim from current `sendEmail`); (b) **then**, wrapped in its own error-isolated block (catch and `Warn`-log, never propagate, never block/delay the branch-(a) result), call `n.notificationService.DispatchEmail(ctx, ...)` using the **recording-adapter-backed** service instance with the identical `DispatchEmailRequest` shape as Branch 1.
- Branch 3 (else — both flags `false`): perform `n.delivery.Send(...)` exactly as today — **byte-identical to current production behavior**, zero risk of regression for the default/pre-rollout state.
- `TriggeredByUserID` note: the current `sendEmail` signature has no actor/user-ID parameter (the legacy `DeliveryMessage` doesn't need one). `DispatchEmailRequest.TriggeredByUserID` is descriptive/audit-only (per its doc-comment, "the actor... who caused the email"); use `"system"` (the documented sentinel value per the field's own comment in `email_dispatch_contracts.go:33`) since adhoc notifications are system-triggered side effects of a state transition, not direct user actions — this requires no new data threading through `ProposalDTO`/`MemberInfo`.

**Step 5 — Update the 4 `Notify*` methods to pass `recipientMembershipID` and `eventType` into `sendEmail`**
Each of `NotifyFocalsForReview`, `NotifyControllerForReview`, `NotifyCreatorApproved`, `NotifyCreatorRejected` already has the recipient's `MemberInfo` in scope (`MemberInfo.MembershipID` already exists, confirmed in `contracts.go:89` — no struct changes needed). Extend each call site's arguments to `sendEmail` with `recipient.MembershipID` and the literal `eventType` string (`"focal_review_requested"`, `"controller_review_requested"`, `"proposal_approved"`, `"proposal_rejected"` respectively — matching each method's existing `templateKey` suffix one-for-one, so no new mapping table is introduced). Update `sendEmail`'s signature to accept these two new parameters.

**Step 6 — Delete `dispatchNotificationAsync`'s goroutine spawn (CF-12 closure)**
In `internal/adhoc/app/service.go`: remove the `go func() { ... fn(context.Background()) ... }()` body of `dispatchNotificationAsync` (current L41-55) and replace each of the 4 call sites (`SubmitProposal` L168, `FocalApprove` L212, `AdminApprove` L320, `Reject` L373) with a direct call to the relevant `n.notifier.Notify*(ctx, ...)`, passing the request-scoped `ctx` already available in each method. Either delete `dispatchNotificationAsync` entirely (if it has no other callers — confirm via grep first) or reduce it to a direct passthrough if retained for call-site uniformity; prefer deletion per "don't keep half-finished abstractions." Run `make check-no-background-context` (the CI grep-assertion named in §AK.5/§AI.2 for CF-12) to confirm zero non-test `context.Background()` matches remain in `internal/adhoc/app/` and `internal/adhoc/infra/`.

**Step 7 — Wire production DI in `server.go`**
Inside the existing `if cfg.WorkflowAdhocEnabled { ... }` block (L435-468):
- Construct `notifRepo := notificationmysql.NewEmailNotificationRepository(pool)`, `outboxRepo := outboxmysql.NewRepository(pool)` (reuse the same construction pattern Batch 2A used in `cmd/worker/main.go`'s `if sqlDB != nil` block — do not duplicate logic, just mirror the call shapes).
- Construct **two** `*notifapp.EmailNotificationService` instances sharing the same `notifRepo`/registry/renderer/idgen/clock but differing only in their terminal `DeliveryAdapter`: one via `WithTransactionalDispatch(pool, outboxRepo)` wired to the existing real `smtpDelivery` (for the `outboxEnabled==true` / authoritative-durable path), and one wired to a new `RecordingDeliveryAdapter` instance (for the `shadowMode==true` / shadow path). *(Note: `EmailNotificationService` does not select its terminal adapter directly — that selection happens inside `EmailDispatchHandler`, constructed by the worker. Confirm during implementation whether the cleanest seam is two `EmailDispatchHandler`-reachable adapter configurations gated by event-payload metadata, or — more likely, and preferable — simply two distinct `IdempotencyKey`-namespaced `DispatchEmailRequest` flows routed to the **same** worker-side handler/adapter, with the "recording" semantics achieved by constructing the **shadow-branch's terminal adapter at the worker** to be the `RecordingDeliveryAdapter` when `cfg.EmailShadowMode` is active program-wide. This wiring-seam decision is the one piece of this plan that requires the implementer to verify the exact construction point against the live `EmailDispatchHandler`/`cmd/worker/main.go` code before writing it — flagged explicitly here so it is not missed, without prescribing an implementation that might not compile against the real seam.)*
- Pass both service instances (or the one service + adapter-selection means, per the resolved seam above), `cfg.EmailShadowMode`, and `cfg.AdhocEmailOutboxEnabled` into the extended `adhocnotif.New(...)` constructor, replacing the current call (L456-458).
- Run `go build ./...` and `go vet ./...` — zero new compile errors expected; this is purely additive wiring inside an existing conditional block.

**Step 8 — Implement `cobo_adhoc_email_shadow_total{outcome, company_id}` emission**
Per the D1-Reassessment-grounded definition (§AK.5 line 657: the metric's detection condition IS `COUNT(*) GROUP BY idempotency_key HAVING COUNT(*) > 1` on `email_notifications`), emit the counter from a query/scan over `email_notifications` rows created by the adhoc module's `DispatchEmail` calls (filterable via `source_aggregate_type = 'ad_hoc_proposal'`, exactly the predicate the §AK.5 verification query already uses): `outcome="match"` for each idempotency key with exactly one row, `outcome="mismatch"` for each with `COUNT(*) > 1`, labeled by `company_id`. (The spec names this metric in §AH.1 row 5 as belonging to "Batch 2A / Batch 2 (Shadow Mode comparison)" — Batch 2A's scope ended at proving the pipeline; Batch 2 is where its adhoc-specific emission is implemented, since only now does adhoc traffic flow through the durable pipeline at all.) Emission point: a lightweight periodic scan (mirroring the cadence/seam of existing reconciliation-style jobs, e.g. `internal/adhoc/reconciliation/daily.go`'s pattern per L328 — but do NOT build a new daily job; a simpler in-process ticker scoped to the Shadow window's lifetime is sufficient and avoids scope creep into Batch 6's reconciliation territory) OR computed directly at `InsertNotificationTx` time via a `FindByIdempotencyKey` pre-check (the service already performs this exact lookup at `email_service.go:92` for replay-safety — the result of that lookup, "did a row already exist for this idempotency key," is *precisely* the match/mismatch signal, obtainable with **zero additional queries**: existing-row-found ⇒ this dispatch would create a `COUNT > 1` ⇒ `outcome="mismatch"`; no existing row ⇒ `outcome="match"`). **Recommend the latter** — it requires no new query, no new job, no new scan cadence, and emits the signal at the exact moment it becomes knowable, with the comparison computed from data the pipeline already produces as part of its proven replay-safety logic.

**Step 9 — Update `docs/ai-cache/reusable-task-updates.md`**
Per `docs/ai-cache/README.md`'s mandatory workflow ("Sau mỗi task: ghi tóm tắt"), append a dated entry summarizing the Batch 2 implementation once Steps 1-8 are code-complete and tests pass.

---

## Test Plan

### Unit Tests
- `notifier_test.go` (extend or create): for each of the 3 routing branches in `sendEmail` — assert Branch 3 (`shadowMode=false, outboxEnabled=false`) calls `delivery.Send` exactly once and never touches `notificationService` (regression guard: byte-identical to pre-Batch-2 behavior); assert Branch 1 (`outboxEnabled=true`) calls `notificationService.DispatchEmail` exactly once with the correctly-formatted `IdempotencyKey` and never calls `delivery.Send`; assert Branch 2 (`shadowMode=true`) calls **both** `delivery.Send` (real) and `notificationService.DispatchEmail` (shadow), and that an injected error from the shadow-side `DispatchEmail` does not prevent or alter the outcome of the `delivery.Send` call (error-isolation contract).
- `RecordingDeliveryAdapter` unit test: `Send` always returns `(DeliveryResult{Provider: "shadow", ProviderMessageID: <deterministic>}, nil)`; never errors; captures/logs the input `DeliveryMessage` fields.
- `service_test.go` (adhoc app): assert the 4 call sites pass the live request `ctx` (not `context.Background()`) into `notifier.Notify*` — e.g. via a `ctx`-value-propagation probe, mirroring how CF-12 fixes are typically asserted elsewhere in the program.
- `config_test.go`: assert `AdhocEmailOutboxEnabled` defaults to `false` (mirrors the existing `EmailShadowMode` default assertion at L72-73).

### Integration Tests
- Reuse Batch 2A's proven E2E harness shape (`email_dispatch_e2e_test.go`'s `newEmailDispatchE2EFixture` pattern) against real MySQL: dispatch through `AdhocProposalNotifier` with `outboxEnabled=true`, assert the full `pending → sending → sent` lifecycle lands in `email_notifications`/`email_delivery_attempts` exactly as Batch 2A proved for the generic pipeline — now exercised through the adhoc-specific `IdempotencyKey` format and `adhoc.*` template keys (closes the gap noted in the grounding report that Batch 2A's E2E used `auth.email_verification`, not the adhoc templates).
- Replay-safety check specific to adhoc idempotency keys: dispatch the same `(eventType, proposalID, recipientMembershipID)` tuple twice; assert exactly one `email_notifications` row, no duplicate outbox publish — mirrors Batch 2A's `TestEmailDispatchE2E_TransactionalPublishWorkerDeliversHappyPath` replay-safety assertion, now against the `adhoc.<event_type>.<proposalID>.<recipientMembershipID>` key shape.

### Shadow Mode Tests

**TC-Shadow-01 — Shadow branch sends real legacy email and never sends real durable email**
- Purpose: prove the core D1-Reassessment guarantee — zero duplicate emails to real users during Shadow Mode.
- Setup: `EmailShadowMode=true`, `AdhocEmailOutboxEnabled=false`; `AdhocProposalNotifier` wired with a spy legacy `DeliveryAdapter` and the real durable pipeline terminating in `RecordingDeliveryAdapter`; trigger one `Notify*` call.
- Expected Result: spy legacy adapter's `Send` is called exactly once (the real, user-facing send); `RecordingDeliveryAdapter.Send` is also called exactly once but its `Provider` field in the resulting `email_delivery_attempts` row reads `"shadow"`, never `"smtp"`; **no second real-transport `Send` occurs anywhere**.

**TC-Shadow-02 — Shadow-side failure never blocks or alters the legacy send**
- Purpose: prove the error-isolation contract (config.go's own pre-existing design comment: "Shadow failures must never break the legacy path").
- Setup: same as TC-Shadow-01, but inject a failure into the shadow-side `DispatchEmail` call (e.g., force `FindByIdempotencyKey` or `BeginTx` to error).
- Expected Result: the legacy `delivery.Send` call still succeeds and is observably unaffected (same return value, same timing characteristics, no added latency beyond a bounded, logged, swallowed error); the shadow-side error is logged at `Warn` and never propagates out of `sendEmail` (matches `ProposalNotifier`'s fire-and-forget contract).

**TC-Shadow-03 — `cobo_adhoc_email_shadow_total{outcome="match"}` emitted for a clean dispatch**
- Purpose: prove the metric reproduces the spec's exact operational definition (§AK.5 L657) for the non-collision case.
- Setup: dispatch one unique `(eventType, proposalID, recipientMembershipID)` tuple in shadow mode against real MySQL.
- Expected Result: exactly one `email_notifications` row for that `idempotency_key`; `cobo_adhoc_email_shadow_total{outcome="match", company_id=...}` increments by exactly 1; `outcome="mismatch"` does not increment.

**TC-Shadow-04 — `cobo_adhoc_email_shadow_total{outcome="mismatch"}` emitted on idempotency-key collision**
- Purpose: prove the mismatch detection matches §AK.5 L657's `COUNT(*) GROUP BY idempotency_key HAVING COUNT(*) > 1` condition exactly — the literal acceptance bar for the cutover gate.
- Setup: force two dispatches to resolve to the same `idempotency_key` (e.g., a simulated duplicate-trigger race — mirrors the Concurrency Harness pattern from Batch 1's TC-01/TC-03/TC-05).
- Expected Result: `FindByIdempotencyKey`'s replay-detection returns the existing row (per `email_service.go:92`'s proven logic — Batch 2A guarantees `COUNT(*) == 1` is preserved even under this race); **but** the *shadow comparison signal*, observing that a dispatch attempt arrived for an idempotency key that already had a row, emits `outcome="mismatch"`. (This is intentionally the rare/exceptional path — under correct operation it should never fire; TC-Shadow-04 exists to prove the *detector* works, by deliberately engineering the condition it exists to catch, exactly as Batch 1's fault-injection harnesses deliberately engineer races to prove their guards hold.)

**TC-Shadow-05 — Flag-precedence: `ADHOC_EMAIL_OUTBOX_ENABLED=true` suppresses the shadow branch regardless of `EMAIL_SHADOW_MODE`**
- Purpose: prove the Behavior Matrix's row-4 precedence rule (durable-authoritative wins; shadow branch is never entered post-cutover).
- Setup: `EmailShadowMode=true`, `AdhocEmailOutboxEnabled=true`; trigger one `Notify*` call.
- Expected Result: exactly one `DispatchEmail` call occurs, routed to the **real** `DeliveryAdapter`-backed service instance; `delivery.Send` (legacy) is never called; `RecordingDeliveryAdapter.Send` is never called.

### Rollback Tests

**TC-Rollback-01 — Instant revert via `ADHOC_EMAIL_OUTBOX_ENABLED=false`**
- Purpose: prove the rollback path is clean and immediate, with the legacy path "kept fully intact" (§AE.2/§AK.5).
- Setup: start with `AdhocEmailOutboxEnabled=true` (durable authoritative); flip to `false` (no restart-dependent state); trigger a `Notify*` call.
- Expected Result: the call routes through Branch 2 or Branch 3 per the current `EmailShadowMode` value (Behavior Matrix rows 1-2) — i.e., behaves exactly as if the flag had always been `false`; legacy `delivery.Send` fires for real; no leftover durable-side state blocks or delays the legacy send.

**TC-Rollback-02 — Verification query matches §AK.5's exact rollback check**
- Purpose: prove the operational rollback runbook's verification query (§AK.5 L656) returns the expected zero-collision result against real data produced by this implementation.
- Setup: run the §AK.5-specified query — `SELECT en.email_notification_id, en.status, en.idempotency_key FROM email_notifications en WHERE en.source_aggregate_type='ad_hoc_proposal' AND en.created_at > <flag_flip_time>` cross-referenced against `ad_hoc_proposals` — against a populated STAGING dataset produced by Steps 1-8's implementation.
- Expected Result: `COUNT(*) GROUP BY idempotency_key HAVING COUNT(*) > 1` returns zero rows; cross-reference count matches the control query exactly (mirrors the audit-write verification pattern at L705).

### Regression Tests
- Full `internal/adhoc/...` suite with both new flags at their `false` defaults — must produce **zero behavioral diffs** from the pre-Batch-2 baseline (Branch 3 is byte-identical to current `sendEmail`). This is the most important regression guard: Batch 2's default-deployed state must be indistinguishable from "Batch 2 not yet deployed."
- `go test -race ./internal/adhoc/...` — confirms CF-12's closure introduces no new races (mirrors Batch 1's `go test -race` gate, AJ.1 row 1/2/6).
- `make check-no-background-context` — zero non-test `context.Background()` matches in `internal/adhoc/app/` and `internal/adhoc/infra/` (the CI mechanical proof of CF-12 closure named in §AI.2).
- Existing `TestDispatchEmail_*` suite in `internal/notification/app/` — must remain green (modulo the pre-existing, documented, out-of-scope `TestDispatchEmail_SanitisesAllSensitiveVars` failure, Risk #2 from Batch 2A's report — not this batch's concern to fix).

---

## Deployment Plan

### DEV
- **Feature Flags**: `EMAIL_SHADOW_MODE=false`, `ADHOC_EMAIL_OUTBOX_ENABLED=false` (defaults — Behavior Matrix row 1, byte-identical to current behavior).
- **Verification**: `go build ./...`, `go vet ./...`, full unit + integration suite green; manually toggle each flag combination locally against a throwaway MySQL (mirrors the Batch 2A E2E evidence-gathering setup at `88.216.208.0:21239`) to walk through all 4 Behavior Matrix rows once before promoting.
- **Rollback**: trivial — revert the branch; no deployed state to unwind (DEV has no real users).

### STAGING
- **Feature Flags**: deploy with `EMAIL_SHADOW_MODE=false, ADHOC_EMAIL_OUTBOX_ENABLED=false` first (confirm zero regression under real STAGING traffic for ≥1 stable cycle); then flip `EMAIL_SHADOW_MODE=true` to open the **24-hour STAGING shadow window** (§AE.3) under synthetic traffic.
- **Verification**: continuous monitoring of `cobo_adhoc_email_shadow_total{outcome="mismatch"}` — must read **zero** for the full 24h; spot-check `email_notifications`/`email_delivery_attempts` rows produced by the shadow branch show `Provider="shadow"` (never `"smtp"`) and correct `pending → sending → sent` transitions; confirm legacy emails are the only ones reaching the STAGING test mailboxes (manual mailbox audit — zero duplicates observed).
- **Rollback**: `EMAIL_SHADOW_MODE=false` — instant, returns to byte-identical pre-Batch-2 behavior; any single `outcome="mismatch"` observation **resets the 24h clock** (§AE.3 — investigate and fix-forward before restarting the count; do not proceed to PROD on a dirty window).

### PROD
- **Feature Flags**: only after STAGING's 24h window completes clean — deploy with `EMAIL_SHADOW_MODE=true, ADHOC_EMAIL_OUTBOX_ENABLED=false` to open the **48-hour PROD shadow window** (§AE.3) under real traffic. **At no point in this plan does any flag combination cause a real duplicate email to a real user** — this is the central guarantee the Revised D1 / `RecordingDeliveryAdapter` design exists to provide, and PROD is exactly where that guarantee matters most.
- **Verification**: same `cobo_adhoc_email_shadow_total{outcome="mismatch"} == 0` bar, sustained for the full 48h; run the §AK.5 one-time SQL verification query (TC-Rollback-02's query) at the window's close as the final gate check; confirm AJ.1 row 8 ("`ADHOC_EMAIL_OUTBOX_ENABLED=true` stable in PROD with zero mismatches over the full window") and row 11 ("SMTP Provider Quota & Combined-Load Verified," archived per L527 to `docs/ai-cache/adhoc-evidence/batch-2/smtp-quota-report.md`, jointly reviewed by Backend on-call and the SMTP provider account owner) are both satisfied before proceeding.
- **Cutover**: flip `ADHOC_EMAIL_OUTBOX_ENABLED=true` (Behavior Matrix row 3 — durable becomes sole authoritative sender); monitor closely for ≥1 fully-stable release cycle; only then flip `EMAIL_SHADOW_MODE=false` to formally close the window (§AE.2 — legacy path remains undeleted "until ≥1 fully-stable post-cutover release has passed," its eventual deletion belongs to a later batch's retirement step, not Batch 2).
- **Rollback**: `ADHOC_EMAIL_OUTBOX_ENABLED=false` — instant revert to legacy-real-send (Behavior Matrix row 1 or 2 depending on `EMAIL_SHADOW_MODE`'s concurrent state); per §AK.5 verbatim, this is "instant revert to the legacy goroutine + `DeliveryAdapter.Send` path, kept fully intact."
- **Mandatory gate** (per `docs/ai-cache/README.md`): rerun a fresh Docker build and report the result (or `BLOCKED:` with reason) after Steps 1-8 land, before any STAGING/PROD promotion.

---

## Rollback Plan

| | |
|---|---|
| **Rollback Trigger** | Any single `cobo_adhoc_email_shadow_total{outcome="mismatch"}` observation during either shadow window (§AE.3: zero-tolerance — "Any single mismatch... resets the window's clock"); OR any post-cutover anomaly in `email_notifications`/`ad_hoc_proposals` correlation; OR a failed §AK.5 verification-query check at any gate point |
| **Rollback Procedure** | **Pre-cutover** (still in shadow window): set `EMAIL_SHADOW_MODE=false` — stops the shadow branch entirely; legacy continues uninterrupted as it always was authoritative; investigate the mismatch root cause, fix-forward, then restart the window's clock from zero per §AE.3. **Post-cutover**: set `ADHOC_EMAIL_OUTBOX_ENABLED=false` — instant revert to "the legacy goroutine + `DeliveryAdapter.Send` path, kept fully intact" (§AK.5 verbatim — note: "the legacy goroutine" phrasing in the spec predates this batch's CF-12 closure; post-Batch-2, the legacy *call path* through `notifier.go`'s Branch 3 is what's reactivated — functionally identical real-SMTP delivery, now correctly threaded with the live request `ctx` rather than `context.Background()`, which is a strict improvement, not a regression, relative to the spec's rollback description) |
| **Verification Queries** | §AK.5 L656 query (TC-Rollback-02): `SELECT en.email_notification_id, en.status, en.idempotency_key FROM email_notifications en WHERE en.source_aggregate_type='ad_hoc_proposal' AND en.created_at > <flag_flip_time>` cross-referenced against `SELECT proposal_id, status FROM ad_hoc_proposals WHERE updated_at > <flag_flip_time>`; plus `SELECT idempotency_key, COUNT(*) FROM email_notifications GROUP BY idempotency_key HAVING COUNT(*) > 1` (the literal L657 detection condition) |
| **Expected Result** | Zero idempotency-key collision groups; cross-reference row counts match the control query exactly (no orphaned/missing notifications); `cobo_adhoc_email_shadow_total{outcome="mismatch"} == 0` for the rolled-back period; legacy delivery resumes immediately and is observably indistinguishable (modulo the `ctx`-threading improvement above) from pre-Batch-2 behavior |

---

## Risks

### Top Risks

1. **Worker-side adapter-selection seam is not yet concretely verified** (flagged explicitly in Step 7). `EmailDispatchHandler` is constructed by the worker with a single `DeliveryAdapter`; the cleanest mechanism for "the same durable pipeline terminates in different adapters depending on whether this is a shadow-branch or authoritative-branch dispatch" needs to be confirmed against the live `EmailDispatchHandler`/`cmd/worker/main.go` code before implementation — this plan identifies the question precisely but does not prescribe the wiring detail (doing so without reading the exact current handler-construction code would risk prescribing something that doesn't compile, which would itself be a form of premature redesign).
   - **Mitigation**: resolve this as the very first sub-task of Step 7, by reading `EmailDispatchHandler`'s constructor and the worker's registration block, before writing any other DI code; if it requires an approach not anticipated here (e.g., per-event-type adapter routing inside the handler), escalate as a clarification rather than improvising a redesign.
2. **Idempotency-key event-type string must exactly match across `notifier.go` and any future consumer** (e.g., Batch 6's reconciliation, Batch 4's audit joins). A typo or inconsistent casing in the four `eventType` literals (`focal_review_requested`, `controller_review_requested`, `proposal_approved`, `proposal_rejected`) would silently produce keys that never collide with their intended counterpart, masking real duplication.
   - **Mitigation**: define the four strings as named constants co-located with the existing template-key constants (which already use the matching suffixes) so a single source of truth drives both; cover with the unit test in Step 5/TC list asserting `idempotencyKey` format per event.
3. **Shadow-branch latency could measurably slow the legacy send if implemented synchronously in sequence** (Branch 2 calls `delivery.Send` then `DispatchEmail` — if the second call blocks before the first returns, or if error-handling accidentally serializes them with the wrong ordering, the user-facing legacy send could be delayed).
   - **Mitigation**: TC-Shadow-02 explicitly asserts the legacy send's timing/outcome is unaffected by shadow-side behavior; implementer should perform the legacy `delivery.Send` call **first**, fully complete it, and only then attempt the shadow dispatch (sequential-but-ordered, not concurrent — concurrency would reintroduce a CF-12-like `ctx`/goroutine concern that this very batch is closing elsewhere). A bounded, swallowed-error shadow attempt after the real send completes adds negligible latency to the user-facing path.

### Mitigation Summary
All three top risks are addressable with information-gathering (Risk 1), naming-discipline + tests (Risk 2), and ordering discipline + tests (Risk 3) — none require a design change to this plan's architecture.

### Residual Risks
- **Thin SMTP-wiring-confidence gap** (carried over from D1 Reassessment, accepted as residual): the shadow window does not exercise the real `DeliveryAdapter` from inside the new DI path. This is closed by the existing, shared, already-production-proven `notificationsmtp.NewAdapter` (same construction pattern, same `cfg.SMTP*` config as the legacy adhoc and reminder pipelines) plus a one-time integration/smoke check reusing the `internal/notification/infra/smtp/adapter_test.go` seam — not by live duplicate sends. This residual risk was explicitly weighed in the D1 Reassessment and judged to be outweighed by the costs of the alternative (Option A).
- **SMTP provider combined-load** (AJ.1 row 11, L527): once cutover completes and the durable pipeline becomes authoritative, adhoc email volume joins `notification.dispatch` volume on the same SMTP provider quota. This is explicitly gated as its own release-gate row with a dedicated 7-day combined-load observation report — not a Batch 2 implementation concern, but a Batch 2 *deployment-gate* dependency that must be tracked to closure before the final cutover is considered durable.

---

## Codex Package

### 1. Scope Summary
Migrate `AdhocProposalNotifier` (4 `Notify*` methods, `internal/adhoc/infra/notification/notifier.go`) and `dispatchNotificationAsync` (`internal/adhoc/app/service.go`) from the legacy fire-and-forget `DeliveryAdapter.Send` path to the durable `EmailNotificationService.DispatchEmail` pipeline (proven complete in Batch 2A). Add a `RecordingDeliveryAdapter` and two config flags (`EMAIL_SHADOW_MODE` reuse, new `ADHOC_EMAIL_OUTBOX_ENABLED`) implementing a 3-way routed Shadow Mode that sends zero duplicate emails to real users (per the authoritative D1 Reassessment). No new pipeline, no new migration, no Batch 2A changes.

### 2. File Map
*(reproduced from the File Map section above — authoritative list)*
- **Must Change**: `internal/adhoc/infra/notification/notifier.go`, `internal/adhoc/app/service.go`, `internal/platform/config/config.go`, `internal/httpserver/server.go`
- **Must Create**: `internal/adhoc/infra/notification/recording_adapter.go` (path/name indicative — match package `notification`'s existing file-naming convention)
- **Must Not Change**: everything under `internal/notification/...` (Batch 2A territory), all migrations, `internal/adhoc/observability/...`, `internal/adhoc/app/contracts.go`

### 3. Contract Changes
- `AdhocProposalNotifier.New(...)`: additive constructor parameters — `notificationService *notifapp.EmailNotificationService` (or the resolved dual-service/adapter-selection shape from Step 7), `shadowMode bool`, `outboxEnabled bool`. No removal of existing parameters; existing callers (only `server.go`'s single construction site) must be updated in lockstep.
- `sendEmail(...)`: additive parameters `recipientMembershipID string`, `eventType string`. Internal rewrite to 3-way branch; external behavior for the default flag-state (both `false`) must be **provably byte-identical** to current behavior (regression test mandatory).
- `Config` struct: additive field `AdhocEmailOutboxEnabled bool`.
- New type `RecordingDeliveryAdapter` implementing `notifapp.DeliveryAdapter` — `Send(ctx, DeliveryMessage) (DeliveryResult, error)`, always `(synthetic-success, nil)`.
- `DispatchEmailRequest.IdempotencyKey` format for adhoc callers: `fmt.Sprintf("adhoc.%s.%s.%s", eventType, proposalID, recipientMembershipID)` — `eventType ∈ {focal_review_requested, controller_review_requested, proposal_approved, proposal_rejected}`.

### 4. Test Matrix
| ID | Type | Asserts |
|---|---|---|
| Unit (notifier) | Unit | 3-way branch routing correctness, error isolation |
| Unit (RecordingDeliveryAdapter) | Unit | deterministic synthetic success, no transport |
| Unit (service ctx-threading) | Unit | CF-12 closure — live `ctx`, not `context.Background()` |
| Unit (config default) | Unit | `AdhocEmailOutboxEnabled` defaults `false` |
| Integration (adhoc E2E via durable pipeline) | Integration | full lifecycle through adhoc-specific idempotency keys + templates |
| Integration (replay-safety) | Integration | duplicate-tuple dispatch → exactly one row |
| TC-Shadow-01 | Shadow | exactly one real send, zero duplicate real sends |
| TC-Shadow-02 | Shadow | shadow failure never blocks/alters legacy send |
| TC-Shadow-03 | Shadow | `outcome="match"` emitted correctly |
| TC-Shadow-04 | Shadow | `outcome="mismatch"` emitted correctly (engineered collision) |
| TC-Shadow-05 | Shadow | flag-precedence — cutover suppresses shadow branch |
| TC-Rollback-01 | Rollback | instant clean revert via flag flip |
| TC-Rollback-02 | Rollback | §AK.5 verification query passes against real data |
| Regression (default-flags) | Regression | byte-identical to pre-Batch-2 behavior |
| Regression (`-race`, CF-12 grep) | Regression | no new races; zero `context.Background()` in adhoc packages |

### 5. Rollback Checklist
- [ ] `ADHOC_EMAIL_OUTBOX_ENABLED=false` flips cleanly with zero deploy/restart-order dependency
- [ ] Legacy `delivery.Send` path fires for real, immediately, post-flip
- [ ] §AK.5 L656/L657 verification queries return zero-collision results
- [ ] No `email_notifications` rows are left in a stuck/ambiguous state by the flip
- [ ] `cobo_adhoc_email_shadow_total{outcome="mismatch"}` reads zero for the rolled-back window

### 6. Deploy Checklist
- [ ] DEV: both flags `false` — full suite green, manual 4-row matrix walkthrough done
- [ ] STAGING: both flags `false` for ≥1 stable cycle, then `EMAIL_SHADOW_MODE=true` → 24h clean window (zero mismatches)
- [ ] PROD: `EMAIL_SHADOW_MODE=true`, `ADHOC_EMAIL_OUTBOX_ENABLED=false` → 48h clean window (zero mismatches) + AJ.1 row 11 SMTP quota report archived
- [ ] Cutover: `ADHOC_EMAIL_OUTBOX_ENABLED=true`, monitor ≥1 stable release
- [ ] Close window: `EMAIL_SHADOW_MODE=false`
- [ ] Fresh Docker build rerun + reported (mandatory gate, `docs/ai-cache/README.md`)
- [ ] `docs/ai-cache/reusable-task-updates.md` entry appended

---

## Final Verdict

## **READY FOR IMPLEMENTATION**

All architectural decisions material to Batch 2's caller-migration scope are locked (per the D1 Reassessment's authoritative Revised D1, the Canonical Decision Record's D2/D3, and this plan's resolution of the remaining concrete design questions — idempotency-key event-type strings, `RecordingDeliveryAdapter` shape/lifecycle, metric-emission seam, branch-ordering for error isolation). The single open implementation-detail question — the precise worker-side seam for selecting between the real and recording terminal adapters (Risk #1 / Step 7's flagged note) — is a **concrete, narrowly-scoped, code-reading task** (read `EmailDispatchHandler`'s constructor + `cmd/worker/main.go`'s registration block before writing the DI code), not an open architectural question requiring further stakeholder input; it is explicitly sequenced as the first action of Step 7 so it cannot be missed or improvised around. Every other step in this plan is concrete enough to implement directly: exact files, exact functions, exact signatures, exact test IDs with Setup/Expected-Result, exact flag matrix, exact rollback queries (quoted verbatim from the spec). No code, no diff, and no Batch 2A re-review were performed in producing this plan, per its constraints.
