# Implementation Reconciliation Review
**Date:** 2026-06-09 | **Persona:** Principal Solution Owner + Principal Architect + Principal Reviewer + Principal Delivery Manager

---

## 1. Executive Summary

Chương trình adhoc email notification đã trải qua 7 phase kế tiếp nhau: Initial Architecture → Batch 1 → Batch 2A → Batch 2 (routing/shadow) → Batch 2B → Batch 3 Cutover → P0 Email Contract Fix → Reminder Reliability Hardening → Email UX Fix. Delivery pipeline durable (`ADHOC_EMAIL_OUTBOX_ENABLED=true`) đang live trên DEV với 6 email thật được gửi thành công. QA validation cho P0 fixes: **PASS WITH RISKS**.

Tuy nhiên, 5 drift/gap quan trọng làm cho hệ thống **chưa đạt READY FOR RELEASE** cho PROD theo tiêu chuẩn plan gốc:
1. **Shadow window bị rút ngắn** — plan yêu cầu 24h STAGING + 48h PROD zero-mismatch; thực tế ~10 phút quan sát trên DEV.
2. **Step 8 (cobo_adhoc_email_shadow_total metric)** — không có evidence implement.
3. **Worker /metrics endpoint** — counter `cobo_email_delivery_total` registered nhưng unscrapeable.
4. **Enterprise admin company CRUD** — GET/PATCH `/api/v1/admin/company` chưa implement (known gap trong CLAUDE.md).
5. **Subscription quota server-side enforcement** — chỉ enforce client-side.

---

## 2. Scope Matrix

### Adhoc Email Program

| Deliverable | Status | Evidence |
|---|---|---|
| **Batch 1**: Legacy templates (4 adhoc templates × vi locale), baseline golden tests | **DONE** | `internal/notification/templates/adhoc.*/vi/`, GoldenFlows baseline |
| **Batch 2A**: EmailNotificationService, WithTransactionalDispatch, outbox worker registration | **DONE** | `batch2a-email-pipeline-wiring-summary.md`, `email_dispatch_e2e_test.go` |
| **Batch 2A**: Synthetic E2E test against real MySQL | **PARTIAL** | Test compiles + passes unit; E2E assertions need MySQL DSN — SKIP in sandbox |
| **Batch 2**: AdhocProposalNotifier 3-way routing (Legacy/Shadow/Outbox) | **DONE** | `notifier.go` rewritten, confirmed via QA validation (ADHOC_EMAIL_OUTBOX_ENABLED=true live) |
| **Batch 2**: CF-12 closure (dispatchNotificationAsync goroutine removed) | **DONE** | QA evidence: emails sent with real ctx, no goroutine leak observed |
| **Batch 2**: RecordingDeliveryAdapter (Revised D1) | **CHANGED** | Original D1 = both real sends. Revised D1 = RecordingDeliveryAdapter (zero duplicate emails). Documented in decision record với audit trail |
| **Batch 2**: ADHOC_EMAIL_OUTBOX_ENABLED config flag | **DONE** | Live on DEV, confirmed via env dump in batch3 cutover doc |
| **Batch 2**: cobo_adhoc_email_shadow_total metric emission (Step 8) | **NOT STARTED** | Không có implementation file, test, hay mention trong bất kỳ completion report |
| **Batch 2**: Shadow window validation (24h STAGING + 48h PROD, zero mismatches) | **CHANGED** | Plan: 24h STAGING + 48h PROD. Actual: ~10 min DEV monitoring, no mismatch observed |
| **Batch 2B**: RetryScheduler interface + outbox retry correctness | **DONE** | `retry_after.go`, `processor.go`, tests pass, docker build exit 0 |
| **Batch 2B**: EmailDispatchHandler DeliveryMetrics interface + Prometheus counter | **DONE** | `cobo_email_delivery_total` wired, handler emits per-attempt outcome |
| **Batch 2B**: Worker /metrics HTTP endpoint for scraping | **NOT STARTED** | Drift documented in Batch 2B record: counter registered-but-unscrapeable |
| **Batch 2B**: Outbox reaper (RequeueStaleProcessing) | **DONE** | Flips processing→pending; worker tick runs reaper; build + tests pass |
| **Batch 3**: Cutover ADHOC_EMAIL_OUTBOX_ENABLED=true on DEV/STAGING | **DONE** | `batch3-cutover-execution-2026-06-09.md`, 6 email thật status=sent |
| **Batch 3**: Alertmanager receiver switch (devnull → email-oncall) | **DONE** | Mailpit drill: [FIRING:1] + [RESOLVED] confirmed |
| **P0 Fix 1**: company_name từ DB (không phải UUID) | **DONE** | QA: "Company X" trong variables_json_sanitized |
| **P0 Fix 2**: creator_name = submitter (không phải reviewer) | **DONE** | QA: "Admin Doanh Nghiep" đúng person |
| **P0 Fix 3**: Reminder deep-link absolute URL (portal_url) | **DONE** | QA: Live SENT occurrence với absolute URL |
| **P0 Fix 4**: PUBLIC_WEB_BASE_URL guard (reject localhost khi ENV != development) | **DONE** | `validatePublicWebBaseURL` active |
| **Reminder Hardening**: SMTP timeout (boundedSendMail, dial 10s, op 30s) | **DONE** | `smtp_sender.go` rewritten; canary against real Gmail STARTTLS pass |
| **Reminder Hardening**: Observability (5 cobo_reminder_* series) | **DONE** | `/metrics` live, 4 alert rules loaded into Prometheus |
| **Reminder Hardening**: Reaper (RequeueStaleDispatching) | **DONE** | Reaper drill: stale DISPATCHING → PENDING in one tick |
| **Reminder Hardening**: Rollback switch (REMINDER_DISPATCH_ENABLED) | **DONE** | Drill: false → emails stop, true → resume, no data loss |
| **Reminder Hardening**: Alert drill (ReminderFailedRecent) | **DONE** | Mailpit: [FIRING:1] 10:51:30 → [RESOLVED] 10:59:00 |
| **Email UX Fix**: proposal_title + proposal_content tách riêng | **DONE** | `enrichProposalForNotification`, `splitChangeNote`, 4 templates + 4 meta.yaml updated |
| **Email UX Fix**: CTA "phê duyệt cuối" → "phê duyệt" | **DONE** | Template source verified |
| **Email UX Fix**: 25 unit + rendering tests | **DONE** | `record_title_test.go`, `notifier_test.go`, `template_render_test.go` |

### IAM Core (P0 → P2)

| Deliverable | Status | Evidence |
|---|---|---|
| **P0**: Bootstrap, migrations, IAM core (users/sessions/credentials) | **DONE** | Summary files p0-1 through p0-7 exist; docker build passes |
| **P0**: CompanyAccess, Authorization (RBAC), Audit, Outbox pattern | **DONE** | API live on DEV, CMS endpoints functional |
| **P1**: Disclosure module (CRUD, workflow, states) | **DONE** | p1-1-disclosure-summary, workflow override implementation confirmed |
| **P1**: Workflow module (deadline, steps, snapshot) | **DONE** | p1-2-workflow-summary, deadline alert work confirmed |
| **P1**: Notification module (templates, rendering) | **DONE** | p1-3-notification-summary, template embed registry functional |
| **P1**: Admin APIs (enterprise admin CRUD) | **PARTIAL** | p1-4-admin-apis-summary exists; **GET/PATCH /api/v1/admin/company MISSING** (CLAUDE.md known gap) |
| **P2**: Projection optimization (effective_access table) | **DONE** | p2-1-projection-summary |
| **P2**: Redis caching (effective_access TTL 5m) | **DONE** | p2-2-redis-summary; in-memory fallback confirmed |
| **P2**: SSO/MFA hooks | **DONE** | p2-3-sso-mfa-hooks-summary |
| **Subscription quota server-side enforcement** | **NOT STARTED** | CLAUDE.md: "Subscription quota hiện chỉ enforce ở client" |

### Frontend (cobo_web_design)

| Deliverable | Status | Evidence |
|---|---|---|
| Login/auth flow, company selection | **DONE** | Live on DEV |
| Portal screens (Dashboard → DisclosureDetail) | **DONE** | Real API integration documented |
| Admin Center (Staff/Org/RBAC/Notifications) | **DONE** | admin-center implementation confirmed |
| CMS (Platform admin: company directory, users, templates) | **DONE** | cms-companies-merge + template editor confirmed |
| Alert Email UX Fix (AlertConfigPanel, permission key fix) | **DONE** | `alert-email-ux-fix-2026-06-01.md`, 17/17 tests pass |

---

## 3. Acceptance Criteria Matrix

### Batch 2 (per implementation plan §AK.5)

| Acceptance Criterion | Status | Evidence |
|---|---|---|
| No wrapper layer introduced | **PASS** | 3-way branch nằm inline trong `sendEmail` |
| Idempotency key format: `adhoc.<event_type>.<proposalID>.<recipientMembershipID>` | **PASS** | Wired in `notifier.go`, confirmed via `email_notifications.idempotency_key` unique constraint |
| No new migration | **PASS** | Batch 2B record: "no schema migration" |
| CF-12 closed (no context.Background() in adhoc packages) | **PASS** | Worker logs: no "notification service nil" warning |
| Shadow window gate: 24h STAGING + 48h PROD, zero mismatches | **FAIL** | Actual: ~10 min DEV, no formal STAGING/PROD shadow window |
| cobo_adhoc_email_shadow_total{outcome} emitted | **NO EVIDENCE** | Step 8 không có trong bất kỳ completion report hay test |
| TC-Shadow-01..05 test coverage | **NO EVIDENCE** | Tests được đặt tên trong plan; không confirm implemented |
| TC-Rollback-01..02 verified | **NO EVIDENCE** | Rollback procedure exists (`.env.bak.cutover.*`) nhưng không có automated test |
| §AK.5 verification query (zero collision groups) | **PASS** | DB evidence: 4 post-fix rows, `idempotency_key UNIQUE`, no duplicates |
| AJ.1 row 11: SMTP combined-load report (7-day) | **NO EVIDENCE** | Không mention trong cutover docs |

### P0 Email Contract Fix

| AC | Status | Evidence |
|---|---|---|
| company_name = business name (not UUID) | **PASS** | QA: "Company X" |
| creator_name = submitter | **PASS** | QA: "Admin Doanh Nghiep" |
| Reminder deep-link absolute | **PASS** | Live SENT at 14:19:22 |
| PUBLIC_WEB_BASE_URL guard | **PASS** | validatePublicWebBaseURL active |

### Email UX Fix

| AC | Status | Evidence |
|---|---|---|
| proposal_title in subject (not change_note) | **PASS** | `subject.txt` × 4 use `{{.proposal_title}}` |
| proposal_title và proposal_content tách riêng trong body | **PASS** | Templates verified, `change_note` removed |
| Content truncated at 300 chars | **PASS** | `splitChangeNote` với `proposalContentMaxLen = 300` |
| CTA text không chứa "cuối" | **PASS** | Test asserts; template verified |
| `change_note` absent from vars | **PASS** | Var builders updated |
| Golden reminder test updated | **PASS** | `portal_url` in vars + golden fixture |

### Reminder Reliability Hardening

| AC | Status | Evidence |
|---|---|---|
| boundedSendMail không block forever | **PASS** | dial 10s + op 30s; canary vs real Gmail |
| 5 cobo_reminder_* series visible trên /metrics | **PASS** | Live confirmed |
| Reaper: DISPATCHING → PENDING khi stale | **PASS** | Reaper drill confirmed |
| REMINDER_DISPATCH_ENABLED kill switch | **PASS** | Drill: false stops email, true resumes |
| Alert drill: FIRING + RESOLVED | **PASS** | Mailpit evidence |

---

## 4. Drift Analysis

### D-01: D1 Shadow Mode — PLANNED vs IMPLEMENTED (Explained, Accepted)

**Planned (Original D1):** Cả 2 path gửi email thật (recipient nhận 2 email trong shadow window).

**Implemented (Revised D1):** Legacy path gửi thật; durable path terminate tại `RecordingDeliveryAdapter` (không gửi email thật, chỉ log).

**Explanation:** D1 Reassessment document (2026-06-08) phân tích §AK.5 line 657 — metric `cobo_adhoc_email_shadow_total` được compute từ `email_notifications.idempotency_key` duplication detection, **upstream** của delivery transport. Signal này reproduceable byte-for-byte với RecordingDeliveryAdapter mà không cần gửi duplicate email thật. Drift này được ghi nhận đầy đủ trong decision record với audit trail.

**Risk:** RecordingDeliveryAdapter chưa prove rằng production DI path (real SMTP adapter) hoạt động đúng dưới load thật — "thin SMTP-wiring-confidence gap" được acknowledge là residual risk.

### D-02: Shadow Window Duration — KHÔNG ĐẠT TIÊU CHUẨN PLAN

**Planned:** 24h STAGING + 48h PROD, zero `cobo_adhoc_email_shadow_total{outcome="mismatch"}`.

**Implemented:** ~10 phút DEV monitoring, 3 snapshots, 6 emails. Không có STAGING shadow run. Không có PROD shadow run trước cutover.

**Explanation:** Batch 3 cutover doc có "Honest note" thừa nhận điều này. Không có giải thích tại sao requirement này được bỏ qua — đây là **unexplained drift**.

**Risk:** HIGH — đây là gate quan trọng nhất của toàn chương trình migration. Bỏ qua nó có nghĩa cutover sang durable pipeline đã xảy ra mà chưa chứng minh zero-mismatch dưới extended real traffic.

### D-03: cobo_adhoc_email_shadow_total Metric — NOT IMPLEMENTED

**Planned:** Step 8 của Batch 2 plan: emit `cobo_adhoc_email_shadow_total{outcome, company_id}` từ `FindByIdempotencyKey` pre-check.

**Implemented:** Không có evidence trong bất kỳ completion report nào. Không có test. Không có file mới.

**Explanation:** Không có.

**Risk:** MEDIUM — metric này là operational gate cho shadow window và cho Batch 6 reconciliation. Thiếu nó nghĩa là không có observability signal để phát hiện idempotency collisions post-cutover.

### D-04: Worker /metrics Endpoint — NOT IMPLEMENTED (Explained)

**Planned:** `cobo_email_delivery_total{outcome,template_key}` scrapeable.

**Implemented:** Counter registered nhưng worker không có HTTP endpoint → unscrapeable.

**Explanation:** Batch 2B record document rõ ràng: "infra outside approved File Map / Phase-0 STOP item." Deferred explicitly.

**Risk:** LOW-MEDIUM — structured log `failed_permanent` vẫn observable qua `docker logs`. Alert sẽ miss nếu chỉ dùng PromQL.

### D-05: TC-Shadow và TC-Rollback Tests — NOT EVIDENCED

**Planned:** 5 TC-Shadow + 2 TC-Rollback tests trong Batch 2 test plan.

**Implemented:** Không có evidence. Completion reports không đề cập. Rollback được test operationally (`.env.bak`) nhưng không có automated test.

**Explanation:** Không có.

**Risk:** MEDIUM — thiếu automated coverage cho routing correctness của 3-way branch.

### D-06: AJ.1 Row 11 — SMTP Combined-Load Report — NOT EVIDENCED

**Planned:** 7-day combined-load observation report sau cutover, archived tại `docs/ai-cache/adhoc-evidence/batch-2/smtp-quota-report.md`.

**Implemented:** File này không tồn tại. Cutover đã xảy ra mà không có report này.

**Explanation:** Không có.

**Risk:** LOW (DEV/STAGING chỉ có synthetic traffic, không phải PROD volume).

---

## 5. Documentation Gaps

| Gap | Severity | Impact |
|---|---|---|
| `docs/ai-cache/adhoc-evidence/batch-2/smtp-quota-report.md` — không tồn tại | LOW | AJ.1 row 11 gate không có artifact |
| Shadow window completion report (24h STAGING) — không có | HIGH | Không có evidence cho gate quan trọng nhất |
| Step 8 implementation record — không có | MEDIUM | cobo_adhoc_email_shadow_total không traceable |
| TC-Shadow-01..05 test files — không confirm tồn tại | MEDIUM | Không thể verify routing correctness coverage |
| Batch 2B remaining-work list — incomplete | LOW | Worker /metrics deferred nhưng không có follow-up ticket |
| `reusable-task-updates.md` entry cho Email UX Fix — cần verify | LOW | Mandatory per README.md |
| QA seed data cleanup — `DELETE FROM reminder_occurrences WHERE occurrence_id = 'qa-step-name-test-*'` — không confirmed done | LOW | SLA_BREACH alert noise mỗi 5s |
| Enterprise admin company CRUD gap — không có implementation ticket | HIGH | Known capability gap, no tracking artifact |

---

## 6. Test Coverage Matrix

| Area | Coverage | Notes |
|---|---|---|
| `EmailNotificationService.DispatchEmail` (transactional path) | **Covered** | `email_dispatch_e2e_test.go` (compiles; needs MySQL to run) |
| Outbox retry correctness (`RetryScheduler`) | **Covered** | `TestProcessor_HonorsRetryAt`, schedule proven without sleep |
| Email dispatch handler DeliveryMetrics | **Covered** | `TestHandler_DeliveryMetricsOutcomes` |
| Outbox reaper (`RequeueStaleProcessing`) | **Covered** | `TestProcessor_RequeueStaleProcessing` |
| `splitChangeNote` (7 cases) | **Covered** | `record_title_test.go` |
| Email UX — title/content separation | **Covered** | `TestEmailUX_TitleAndContentAreSeparateVars`, `TestEmailUX_SingleLineNoteHasNoContent` |
| Email UX — CTA no "cuối" | **Covered** | `TestEmailUX_CTALabelDoesNotContainCuoi` |
| Adhoc template rendering (6 subtests × 4 templates) | **Covered** | `template_render_test.go` |
| Golden email output (7 templates including reminder) | **Covered** | `TestEmailRenderer_GoldenOutputs` |
| Reminder SMTP timeout hardening | **Covered** | `smtp_timeout_test.go` |
| Reminder reaper (stale DISPATCHING → PENDING) | **Covered** | inmemory + service reaper tests |
| Reminder observability collector | **Covered** | `db_collector_test.go`, `_endpoint_test.go` |
| REMINDER_DISPATCH_ENABLED config | **Covered** | `config_test.go` |
| `AdhocProposalNotifier` 3-way routing | **MISSING** | TC-Shadow-01..05 không được confirm implement |
| RecordingDeliveryAdapter unit test | **MISSING** | Không có evidence file tồn tại |
| CF-12 closure (ctx threading, not context.Background()) | **MISSING** | `make check-no-background-context` — không có evidence run |
| cobo_adhoc_email_shadow_total emission | **MISSING** | Step 8 không implement |
| TC-Rollback-01..02 | **MISSING** | Operational rollback tested nhưng không automated |
| `TestDispatchEmail_SanitisesAllSensitiveVars` | **FAILING** | Pre-existing; `auth.user_invitation.new_user_company` missing `support_email` |
| `TestLoad_UserAvatarEnvOverride` | **FAILING** | Pre-existing; Windows backslash path |
| `companyaccess/transport/http` | **FAILING** | Pre-existing |

---

## 7. Remaining Work

### P0 — Blocking cho PROD release của email pipeline

| Item | Effort |
|---|---|
| Implement cobo_adhoc_email_shadow_total metric (Step 8 Batch 2) | Small — seam tại `FindByIdempotencyKey` đã có |
| Add TC-Shadow-01..05 automated tests (hoặc xác nhận chúng đã tồn tại) | Medium |
| Run §AK.5 verification query và archive kết quả | Trivial |
| Fix `TestDispatchEmail_SanitisesAllSensitiveVars` | Small — thêm `support_email` vào test vars |
| Cleanup QA seed occurrence (stale PENDING SLA_BREACH) | Trivial SQL |

### P1 — Blocking cho PROD confidence

| Item | Effort |
|---|---|
| Execute proper shadow window trên STAGING (≥4h, real traffic simulation) | 4–24h soak |
| Implement GET/PATCH `/api/v1/admin/company` (enterprise admin company CRUD) | Medium |
| Worker /metrics HTTP endpoint (scrape port + target config) | Medium |
| AJ.1 row 11: SMTP combined-load report (7 days post-cutover) | Monitoring |

### P2 — Technical Debt

| Item | Effort |
|---|---|
| Subscription quota server-side enforcement (HTTP 402) | Medium-Large |
| RecordingDeliveryAdapter unit test | Small |
| CF-12 CI gate: `make check-no-background-context` | Small |
| Batch 6: Prometheus recording rules + alert rules cho adhoc email pipeline | Medium |
| Batch 7: Historical data repair (DF-02/DF-03/DF-05) — pre-fix UUID rows vẫn còn trong DB | Large |
| REMINDER_* + workflow flags moved to `.env` (dual `-f` footgun) | Small |

### Documentation Debt

| Item |
|---|
| Append Email UX Fix entry to `reusable-task-updates.md` (nếu chưa done) |
| Archive shadow window evidence (batch-2/shadow-evidence/) |
| Create `docs/ai-cache/adhoc-evidence/batch-2/smtp-quota-report.md` |
| Document TC-Shadow test locations |

---

## 8. Production Risks

### RISK-01 — Shadow Window Bypassed (SEVERITY: HIGH)

**Description:** Plan yêu cầu 24h STAGING + 48h PROD zero-mismatch trước khi flip `ADHOC_EMAIL_OUTBOX_ENABLED=true`. Thực tế: cutover xảy ra sau ~10 phút DEV monitoring. Durable pipeline đang là authoritative sender trên DEV mà chưa pass gating validation chính thức.

**Impact:** Nếu có bug latent trong pipeline (template render, idempotency logic, outbox dispatch timing), sẽ không được phát hiện trước khi user thật chịu ảnh hưởng trên PROD.

**Mitigation:** Chạy shadow window trên STAGING trước khi promote lên PROD. Có thể rút ngắn còn 4–8h với synthetic load nếu đồng ý chấp nhận rủi ro thấp hơn full 24h.

### RISK-02 — cobo_adhoc_email_shadow_total Không Có (SEVERITY: MEDIUM)

**Description:** Metric này là observability signal chính cho collision detection. Thiếu nó có nghĩa post-cutover không có automated way để phát hiện idempotency key collisions.

**Mitigation:** Implement Step 8 trước PROD release. Seam đã có tại `email_service.go:92` (`FindByIdempotencyKey`) — effort nhỏ.

### RISK-03 — Gmail Inbox Không Inspectable Trực Tiếp (SEVERITY: LOW)

**Description:** QA validation dùng `variables_json_sanitized` làm proxy cho email body. Email rendering thật (HTML/plain text) trong inbox chưa được visual-inspect.

**Mitigation:** Accepted — SMTP backend là Gmail real, STARTTLS+AUTH confirmed. Variables proxy là valid vì worker dùng chính xác vars đó để render.

### RISK-04 — Pre-existing Test Failures (SEVERITY: LOW)

**Description:** 3 packages fail trên `go test ./...`: `companyaccess/transport/http`, một số tests trong `httpserver`, `TestDispatchEmail_SanitisesAllSensitiveVars`. Đã verify pre-existing trên clean HEAD.

**Mitigation:** Fix `SanitisesAllSensitiveVars` (thêm `support_email` var) trước PROD release.

### RISK-05 — Enterprise Admin Company CRUD Gap (SEVERITY: MEDIUM)

**Description:** Enterprise admin không thể read/update their own company profile qua API. CLAUDE.md đã mark là known gap.

**Mitigation:** Implement `GET /api/v1/admin/company` + `PATCH /api/v1/admin/company` scoped tới `sub.CompanyID`, không expose `verification_status`.

### RISK-06 — Worker Metrics Không Scrapeable (SEVERITY: LOW)

**Description:** `cobo_email_delivery_total` counter không có HTTP endpoint trên worker process → Prometheus không scrape được.

**Mitigation:** `failed_permanent` vẫn có structured log → `docker logs`-based alerting hoạt động. Không blocking cho DEV, cần fix trước PROD.

### RISK-07 — Dual `-f` Deploy Footgun (SEVERITY: LOW)

**Description:** `make deploy-be` chỉ pass `-f docker-compose.artifacts.yml`, bỏ `-f docker-compose.override.yml` → mất `WORKFLOW_ADHOC_ENABLED`, `PERIODIC_SEEDING_ENABLED=true`, etc.

**Mitigation:** Merge workflow/reminder flags vào `.env` thay vì override file.

---

## 9. Release Readiness

### Verdict: **READY WITH RISKS**

#### Đạt tiêu chuẩn READY WITH RISKS vì:
- Delivery pipeline hoạt động end-to-end: 6 email thật, status=sent, no duplicates, no wrong recipients.
- P0 fixes (company_name, creator_name, reminder deep-link, guard): QA PASS WITH RISKS.
- Email UX fix (title/content separation): implemented + 25 tests.
- Reminder hardening (SMTP timeout, reaper, observability, alert rules): deployed + drilled.
- Rollback procedure sẵn sàng: `.env.bak.cutover.*`, `ADHOC_EMAIL_OUTBOX_ENABLED=false` flip < 60s.

#### KHÔNG đạt READY FOR RELEASE (full PROD) vì:
1. **Shadow window bypassed** — gap chưa được explain thuyết phục; gating criterion của plan gốc không có pass evidence.
2. **cobo_adhoc_email_shadow_total metric** — Step 8 không implement; không có detector cho post-cutover collision.
3. **TC-Shadow-01..05 tests** — routing correctness của 3-way branch chưa có automated coverage confirm.
4. **TestDispatchEmail_SanitisesAllSensitiveVars** — pre-existing failure trong `go test ./...` chưa fix.
5. **Enterprise admin company CRUD** — known capability gap không có tracking artifact hay timeline.

#### Điều kiện để upgrade lên READY FOR RELEASE:
- [ ] Implement Step 8 (cobo_adhoc_email_shadow_total) + TC-Shadow tests
- [ ] Fix `TestDispatchEmail_SanitisesAllSensitiveVars`
- [ ] Chạy STAGING shadow window (tối thiểu 4h synthetic traffic, zero mismatch)
- [ ] Archive §AK.5 verification query kết quả
- [ ] Triage/ticket enterprise admin company CRUD gap

---

## 10. Hạng Mục Có Nguy Cơ Đã Bị Quên

Dựa trên cross-reference giữa plan gốc và implementation evidence:

| Hạng mục | Lý do nghi ngờ bị quên |
|---|---|
| **cobo_adhoc_email_shadow_total metric (Step 8)** | Không có mention trong bất kỳ completion report nào. Step 9 (reusable-task-updates entry) được đề cập nhưng Step 8 thì không. |
| **TC-Shadow-01..05 tests** | Test plan rất chi tiết trong Batch 2 plan document nhưng không có file path hay completion note nào. |
| **AJ.1 row 11 — SMTP combined-load report** | Explicitly flagged as "gating row with dedicated 7-day observation." File `docs/ai-cache/adhoc-evidence/batch-2/smtp-quota-report.md` không tồn tại. |
| **Batch 4: Audit-trail semantics cho adhoc emails** | Batch 2 plan explicitly out-scopes sang "Batch 4". Không có Batch 4 doc nào trong `docs/ai-cache/`. |
| **Batch 6: Prometheus recording rules + alert rules cho adhoc email** | §Out of Scope row 6: "Batch 6 territory". Không có Batch 6 doc. |
| **Batch 7: Historical data repair (DF-02/DF-03/DF-05)** | §AK.9 assigns đến Batch 7. Pre-fix email rows (UUID company_name) vẫn còn trong DB. |
| **RecordingDeliveryAdapter unit test** | Plan Step 2: "Write its unit test in the same step." Không có file path nào được mention trong completion reports. |
| **`make check-no-background-context` CI gate** | §AI.2: "CI mechanical proof of CF-12 closure". Không có evidence gate này tồn tại. |
| **Idempotency key constants** | Plan Risk #2: "define four strings as named constants." Không confirm đã làm constant hay inline string literal. |
| **PROD environment readiness** | Toàn bộ chương trình chỉ validated trên DEV (88.216.208.0). Không có STAGING và chưa có PROD environment setup documented. |
