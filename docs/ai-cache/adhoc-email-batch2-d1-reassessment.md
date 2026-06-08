# Batch 2 — D1 Reassessment: Shadow Mode Delivery Semantics

> Date: 2026-06-08
> Persona: Principal Architect + Principal SRE + Principal Messaging Architect
> Scope: re-examine ONLY Decision D1 (Shadow Mode delivery semantics) from `adhoc-email-batch2-decision-record.md` — not a full Batch 2 review
> Trigger: a closer re-read of §AK.5's verification-query block (lines 656-658 of `adhoc-email-spec-v3.md`) surfaces a concrete definition of the `outcome="mismatch"` metric that was not weighed in the original D1 analysis and materially changes the conclusion

---

# Interpretation Review

**New grounding fact (the pivot point of this reassessment) — §AK.5 Rollback verification block, line 657 (verbatim):**

> "Expected Result: exactly one `email_notifications` row per (proposal transition × recipient) — `COUNT(*) GROUP BY idempotency_key HAVING COUNT(*) > 1` returns zero rows (**this is exactly `cobo_adhoc_email_shadow_total{outcome="mismatch"}`'s detection condition**, run as a one-time SQL check at cutover in addition to the continuous shadow-mode counter)"

This is the spec's own, exact, operational definition of what "mismatch" measures — and it is **not** a cross-system comparison ("did legacy's send result match durable's send result"). It is a **single-table, internal duplicate-row detector**: count rows in `email_notifications` (the durable pipeline's own ledger) grouped by `idempotency_key`; any group with `COUNT(*) > 1` is a mismatch. The legacy path's outcome does not appear anywhere in this detection condition — it is computed entirely from the durable pipeline's own persisted state.

**Q1 — Does "side-by-side" in the spec actually mandate both real deliveries?**

No — not once this concrete metric definition is taken into account. "Side-by-side" (L280, L785) is satisfied by **invoking** both pipelines for the same production events under real concurrency/load/replay conditions (so that any latent duplicate-publish bug in the durable side has a realistic chance to manifest as a `COUNT(*) > 1` row group). It does **not** require the durable side's *terminal delivery transport* to be a live SMTP transmission, because the thing being measured — idempotency-key collision count — is fully determined at **insert time** (`InsertNotificationTx`/`PublishEventTx`, the transactional-publish step), strictly upstream of whatever the `DeliveryAdapter` at the tail of the pipeline does. Whether the tail adapter is real SMTP or a recording/shadow adapter has **zero effect** on whether `email_notifications` ends up with duplicate rows for the same idempotency key — that outcome is decided entirely by the caller-side and transactional-publish logic that Batch 2 is migrating.

This reframes the original D1 reasoning: I had read "comparing outcomes" as requiring two independently-completed live deliveries to compare against each other. The spec's own operational definition (L657) shows "outcome" in this context means **"did the durable pipeline's persisted ledger end up with exactly one row per idempotency key"** — an internal correctness/exactly-once check, not a cross-system delivery-result diff. That is a strictly narrower, and fully achievable-without-real-duplicate-sends, validation target.

---

# Option Analysis

**Q2 — Is there an option that compares outcomes without sending duplicate emails to real users?**

Yes: **terminate the durable path at a recording/shadow `DeliveryAdapter`** instead of the real SMTP adapter, while still running the *entire* rest of the durable pipeline for real — `DispatchEmail` → `BeginTx → InsertNotificationTx → PublishEventTx → Commit` → outbox → worker `Tick` → `EmailDispatchHandler` → template resolution/rendering → retry/backoff scheduling → status transitions (`pending → sending → sent/retry/failed_permanent`). Every one of these stages is exercised exactly as in production; only the final "transmit bytes over SMTP" step is swapped for a recorder that captures the fully-resolved message (recipient, subject, rendered body, template key) and reports a synthetic terminal outcome. The `cobo_adhoc_email_shadow_total{outcome="mismatch"}` signal — per L657's own definition — is computed from `email_notifications.idempotency_key` row counts, which this approach populates identically to a real-SMTP-terminated run. **The metric the spec actually gates on is fully and identically produced.**

**Q3 — Does a fake/shadow adapter at the tail still meet the validation goal?**

Yes — and there is direct, accepted precedent for exactly this pattern *in this same codebase, for this same pipeline, at an equal-or-higher validation bar*: Batch 2A's own `TestEmailDispatchE2E_*` suite (the evidence that earned **Batch 2A ACCEPTED** with **Final Verdict: PASS**) wired the genuine production chain — `EmailNotificationService → real outboxmysql.Repository → real platformoutbox.Processor.Tick → the exact wrapper-closure registered in cmd/worker/main.go → EmailDispatchHandler` — and terminated it in a `fakeAdapter`, explicitly noting "*only the SMTP transport is faked; every persistence and routing layer is the genuine production implementation*." That was accepted as sufficient to prove AK.4's acceptance criterion (full `pending → sending → sent/retry/failed_permanent` lifecycle, transactional publish, retry/backoff — a **higher** bar than Shadow Mode's narrower idempotency-collision check). If a fake terminal adapter was sufficient to prove the pipeline *works*, it is necessarily sufficient to prove the pipeline *does not duplicate rows* — duplication is decided upstream of the adapter, not by it.

What a shadow adapter does **not** prove: that the real `notificationsmtp.NewAdapter`/SMTP transport, invoked from inside the new code path, successfully transmits to a live mailbox. But this is **not a novel risk surface that Batch 2 introduces** — it is the exact same adapter type, constructed with the exact same `cfg.SMTP*` config, that the **legacy adhoc path already uses today** (`internal/httpserver/server.go` L440-500: `notificationsmtp.NewAdapter(notificationsmtp.Config{Host: cfg.SMTPHost, ...})`, the comment explicitly noting "*same SMTP config as reminder module*"). It is shared, already-in-production, already-proven infrastructure. The only incremental thing "real send via durable path" would prove over a shadow-adapter run is "the constructor was wired correctly in the new DI graph" — a wiring-correctness concern, cheaply and deterministically verifiable by a one-time startup smoke test or the existing `internal/notification/infra/smtp/adapter_test.go` seam (already reused by the SMTP-Mock Harness per L447), not by 72 cumulative hours of duplicate live sends to real users.

**Bonus finding — real dual-delivery would actively *pollute* the very metric it's meant to validate.** If both sides perform independent live SMTP transmissions, each is independently subject to transient provider variance (rate-limiting, momentary `4xx`, network jitter — the same `transient_smtp` class the retry policy exists to absorb). Two independent live attempts for the same logical event can easily diverge in outcome for reasons that have **nothing to do with** the durable pipeline's correctness — yet L657's detection condition is a strict row-count check with **zero tolerance** ("Any single mismatch... resets the window's clock," L282). A shadow-adapter approach removes this entire class of false-positive risk, because the recorder's outcome is deterministic given the same rendered input — collisions detected are real collisions, not transport noise.

| | A — both real sends | Revised — durable shadow-adapter |
|---|---|---|
| Validates transactional publish, outbox routing, retry/backoff, idempotency, template rendering, status tracking | ✅ | ✅ (identically — these all happen upstream of the adapter) |
| Produces the exact `outcome="mismatch"` signal per L657's definition | ✅ | ✅ (identical — computed from `email_notifications`, not from delivery transport) |
| Validates real-SMTP wiring inside the new DI path | ✅ | ❌ (but this is shared/already-proven infra — see above; cheaply covered by a smoke test) |
| Duplicate emails to real users (24h+48h) | Yes — by design | None |
| Risk of false-positive mismatches from independent SMTP transient variance resetting the 24h/48h clock | Yes (real, structural) | None |
| Requires Product Owner sign-off to deliberately double-send to real compliance-platform users | Yes | No |

**Q4 — Does the business cost of duplicate email outweigh the technical value gained?**

Yes. The audience here is not anonymous consumers — it is **focal points, controllers, and proposal creators inside regulated, publicly-listed companies**, receiving compliance-workflow notifications ("your disclosure proposal was approved/rejected/needs review") from a CBTT compliance platform. Two emails per lifecycle event, sustained for up to 72 cumulative hours, reads as a reliability defect to exactly the audience whose trust in the platform's correctness matters most — and could plausibly trigger support tickets, duplicate replies/actions, or doubt about which notification is authoritative. Weighed against that recurring, user-facing, reputationally-loaded cost, the *marginal* technical value "real send" adds over a shadow-adapter run is thin: it amounts to re-confirming that an adapter type already proven in production-by-the-legacy-path-itself still works when constructed a second time in a new wiring location — a check whose natural home is a one-time integration/smoke test, not a 72-hour live double-send campaign. **The cost clearly outweighs the marginal value.**

**Q5 — If Option A is retained, does it need separate Product Owner approval?**

Yes, unambiguously — and this question's very inclusion in the brief is itself a signal that engineering should not be the sole authority here. Deliberately sending duplicate user-facing compliance notifications to real regulated-entity officers, for up to 72 cumulative hours, in production, is a product/business decision with reputational and stakeholder-perception consequences that sit outside engineering's unilateral remit. It would require explicit, documented PO sign-off — captured as a release-gate artifact — *before* the window opens, not after an issue surfaces. (Notably: the revised shadow-adapter approach removes this approval dependency entirely, which is itself an argument in its favor — it keeps the cutover on engineering's critical path without adding an external approval gate that could stall the canonical Batch 0→7 sequence.)

---

# Recommended Decision

**Replace Option A with: "Durable shadow-adapter" — full real pipeline execution, recording terminal adapter.**

- **Legacy path:** unchanged — runs for real, remains the system of record and the sole channel actually reaching users' mailboxes throughout the Shadow window (consistent with L785: "the legacy path remains fully intact... until ≥1 fully-stable post-cutover release has passed").
- **Durable path:** `notifier.go` invokes `NotificationService.DispatchEmail(...)` for real, exercising the complete genuine chain — transactional publish, outbox, worker `Tick`, `EmailDispatchHandler`, template rendering, retry/backoff, status tracking — exactly as Batch 2A's accepted E2E proof did, but the `DeliveryAdapter` at the tail is a **recording/shadow adapter** (the same `fakeAdapter`-class seam Batch 2A already built and got accepted for an equal-or-higher bar) that captures the fully-resolved message and reports a synthetic terminal outcome instead of transmitting over live SMTP.
- **Comparison metric:** `cobo_adhoc_email_shadow_total{outcome="match"|"mismatch"}` is computed **exactly as L657 defines it** — `SELECT idempotency_key, COUNT(*) FROM email_notifications ... GROUP BY idempotency_key HAVING COUNT(*) > 1` — which this approach satisfies identically to a real-SMTP-terminated run, because the signal is generated entirely upstream of the adapter.
- **SMTP-wiring confidence:** covered separately and far more cheaply by a one-time integration/smoke check reusing the `internal/notification/infra/smtp/adapter_test.go` seam (already the SMTP-Mock Harness's foundation per L447) — confirming the new DI construction site produces a working adapter, without needing live duplicate sends to prove it.

This keeps "side-by-side" (both pipelines invoked, under real concurrency, for the same events — satisfying Q1's narrower, metric-grounded reading), keeps the gating metric byte-for-byte identical to the spec's own operational definition, and eliminates the duplicate-send cost, the false-positive-mismatch risk, and the PO-approval dependency — all in one substitution that touches only the adapter wired into the durable path during the Shadow window (a configuration/construction-site concern, not a redesign of any pipeline logic).

---

# Risk Assessment

| Risk | Under Option A (real dual-send) | Under Revised (shadow-adapter) |
|---|---|---|
| Duplicate emails to real compliance-platform users (reputational/UX cost, ~72h cumulative) | **High — accepted by design** | None |
| False-positive `outcome="mismatch"` from independent SMTP transient variance, resetting the 24h/48h clock (L282: zero tolerance, any single mismatch resets) | **Real, structural — could indefinitely delay cutover for reasons unrelated to code correctness** | Eliminated — recorder outcome is deterministic given identical rendered input |
| Gap in proving real-SMTP-from-new-DI-path wiring | None | Small — but closes against shared, already-production-proven infra (`notificationsmtp.NewAdapter`, same config as legacy/reminder); cheaply closed by a smoke test reusing an existing harness seam |
| Requires external (Product Owner) approval before the window can open — adds a non-engineering dependency to the Batch 0→7 critical path | **Yes** | No |
| Deviates from literal spec wording ("side-by-side", "comparing outcomes") | No literal deviation if "outcome" is read per the spec's own L657 operational definition (durable-internal duplicate-row count) — which this approach reproduces identically | Same — no deviation, because the gated metric is reproduced exactly |
| Consistency with precedent already accepted in this program | Introduces a new live-duplicate-send pattern with no precedent | Directly mirrors the `fakeAdapter` pattern Batch 2A already built and that earned **Final Verdict: PASS / Batch 2A ACCEPTED** |

No new risk surfaces are introduced by the revised approach beyond the one explicitly named above (thin SMTP-wiring-confidence gap), and that gap has a cheap, concrete, already-precedented closure path that does not require touching user-facing behavior.

---

# Final Verdict

## **REVISE D1**

The original D1 (`Option A — Legacy send thật + Durable send thật`) was grounded in a literal reading of "side-by-side" + "comparing outcomes" that, on closer inspection of the spec's **own concrete operational definition of the gated metric** (§AK.5 line 657: `cobo_adhoc_email_shadow_total{outcome="mismatch"}` = `COUNT(*) GROUP BY idempotency_key HAVING COUNT(*) > 1` on `email_notifications`), turns out to be **narrower and fully satisfiable without real duplicate deliveries**. The signal the spec actually gates the cutover on is an internal duplicate-row check on the durable pipeline's own ledger — generated entirely upstream of the delivery-transport step — and is reproduced byte-for-byte identically whether the terminal adapter is real SMTP or a recording/shadow adapter of the exact class Batch 2A already built and got **ACCEPTED** for a higher bar.

**Revised D1:** Shadow Mode runs the legacy path for real (unchanged, remains system of record) and the durable path through its complete genuine pipeline terminating in a **recording/shadow `DeliveryAdapter`** — producing the identical gating metric, eliminating duplicate user-facing sends, eliminating the false-positive-mismatch risk from independent SMTP variance, and removing the dependency on a separate Product Owner approval that real duplicate-sending would otherwise require.
