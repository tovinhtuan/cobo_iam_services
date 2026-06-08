# Batch 2 — Grounding Report: First Production Caller of `EmailNotificationService`

> Date: 2026-06-08
> Persona: Principal Backend Engineer + Principal Solution Architect + Principal Messaging Architect
> Source of Truth (read in order): `adhoc-email-spec-v3.md` (cobo_web_design/docs/ai-cache/) → Batch 1 Completion Report → Batch 5(a) Completion Report → Batch 2A Completion Report → Batch 2A E2E Execution Evidence Report
> Current Status: Batch 0 PASS, Batch 1 PASS, Batch 5(a) PASS, Batch 2A ACCEPTED. Batch tiếp theo: Batch 2.
> Constraint honored: KHÔNG code. KHÔNG tạo implementation plan. KHÔNG redesign. Không suy đoán — mọi kết luận grounded vào spec §AK.5 + code thực tế.

---

# Current Caller Inventory

Bốn pipeline gửi email tồn tại song song, không hợp nhất, trong cùng codebase:

**1. Adhoc (`AdhocProposalNotifier`) — Batch 2 target**
- Current Caller: `internal/adhoc/app/service.go` — 4 call sites (`SubmitProposal` L168, `FocalApprove` L212, `AdminApprove` L320, `Reject` L373), mỗi call wrap trong `dispatchNotificationAsync` (L41-55, spawn `go func(){ ... fn(context.Background()) }`)
- Path: `service.go` → `dispatchNotificationAsync` (goroutine + `context.Background()`) → `AdhocProposalNotifier.Notify*` (4 methods, `notifier.go`) → `sendEmail` (L128-159) → `n.delivery.Send(ctx, notifapp.DeliveryMessage{...})` — gọi trực tiếp `DeliveryAdapter.Send`, đồng bộ, lỗi chỉ log qua `n.log.Warn`, không retry/outbox
- Purpose: thông báo proposal review/approval cho focal/controller/creator (4 template: `adhoc.controller_review_requested`, `adhoc.focal_review_requested`, `adhoc.proposal_approved`, `adhoc.proposal_rejected`)
- DI wiring: `internal/httpserver/server.go` L440-500 — `smtpDelivery := notificationsmtp.NewAdapter(...)`, `proposalNotifier = adhocnotif.New(inAppSvc, smtpDelivery, emailTemplateRegistry, emailRenderer, cfg.PublicWebBaseURL, log)`

**2. Auth/IAM (`publishEmail` → outbox → `deliverAuthEmailEvent`)**
- Current Caller: `internal/iam/app/service.go` — `s.renderEmailContent(...)` rồi `s.publishEmail(ctx, eventType, userID, payload)` (L298-311, L789-828); `publishEmail` (L901-909) chỉ `s.outbox.Publish(ctx, events.Event{EventType: eventType, Payload: payload, ...})`
- Path: `iam/app/service.go` → outbox (`auth.password_reset_requested`, `auth.admin_password_reset_requested`, `auth.user_invitation_sent`, `auth.email_verification_requested`) → `cmd/worker/main.go` L80-90 routes cả 4 event type này tới `deliverAuthEmailEvent` (L253-273) → unmarshal payload lấy raw `to`/`subject`/`body` string → `sendSMTPMail` → `smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))` raw, **bỏ qua hoàn toàn** templates/renderer/`notifapp`/outbox-retry machinery của module notification
- Purpose: email xác thực/mời người dùng/đặt lại mật khẩu trong luồng IAM

**3. Reminder (`EmailSender.SendReminderEmail`)**
- Current Caller: `internal/reminder/app/service.go` L396 — `msgID, sendErr := s.emailSender.SendReminderEmail(ctx, templateCode, req.TemplatePayload, validEmails, req.IdempotencyKey)`, gọi đồng bộ trong request path
- Path: `EmailSender` interface (L19, L30-33) → `WithEmailSender` option (L36-39) → wired bằng `reminderemail.NewSMTPSender(reminderemail.SMTPConfig{...})` tại `cmd/worker/main.go:159` và `internal/httpserver/server.go:400` → gửi trực tiếp qua SMTP, không outbox, không retry framework
- Purpose: nhắc deadline công bố thông tin (CBTT)

**4. New durable pipeline (`EmailNotificationService.DispatchEmail`) — built Batch 2A, ZERO production caller**
- Current Caller: **none** — confirmed bằng grep, `internal/notification/app/email_service.go` là nơi DUY NHẤT construct `EmailNotificationService`; zero non-test call site của `.DispatchEmail(`
- Path (đã hoàn thiện end-to-end, chứng minh bằng E2E test thật chạy live MySQL — xem `batch2a-e2e-execution-evidence-summary.md`): `EmailNotificationService.DispatchEmail` (với `WithTransactionalDispatch`) → `BeginTx → InsertNotificationTx → PublishEventTx → Commit` → `outbox_events` (`email.dispatch`) → `cmd/worker/main.go` L93-117 (gated `if sqlDB != nil`) registers wrapper closure `event.PayloadJSON → EmailDispatchHandler.Handle` → `EmailDispatchHandler` → `DeliveryAdapter.Send` → tracking trong `email_notifications`/`email_delivery_attempts` với full retry/backoff
- Purpose: pipeline durable đích — đã wired đầy đủ ở Batch 2A nhưng "constructed and reachable, proven only by the synthetic test", chưa có caller production thật nào trỏ vào nó

**Phát hiện đáng chú ý — `notification.dispatch` outbox handler là no-op stub**: `cmd/worker/main.go` L74-79 đăng ký `processor.Register("notification.dispatch", ...)`, nhưng handler body chỉ unmarshal payload, log `"dispatch notification event"`, rồi `return nil` — **không deliver gì cả**. Đây là một event type khác (`notification.dispatch` ≠ `email.dispatch`), đăng ký "không lỗi" nhưng không bao giờ gửi thư thật.

# Batch 2 Target

Trích §AK.5 — Batch 2: Durable Pipeline Cutover (verbatim, không suy đoán — spec nêu đích danh):

> Must-Change: `internal/adhoc/infra/notification/notifier.go` — "replace `DeliveryAdapter.Send` goroutine calls with `NotificationService.DispatchEmail(ctx, DispatchEmailRequest{..., IdempotencyKey: fmt.Sprintf("adhoc.%s.%s.%s", eventType, proposalID, recipientMembershipID)})`, calling `DispatchEmail` directly — no wrapper of any kind is introduced or required"
>
> Must-Change: `internal/adhoc/app/service.go` — "delete `dispatchNotificationAsync`'s `context.Background()` goroutine spawn at `service.go:43` — pass `ctx` through directly; this single deletion structurally resolves CF-12"
>
> Must-Update-Config: `ADHOC_EMAIL_OUTBOX_ENABLED` — `boolEnv("ADHOC_EMAIL_OUTBOX_ENABLED", false)`; "entry-gated by `EMAIL_SHADOW_MODE`, already registered"
>
> Must-Update-Migration: none

Đối chiếu codebase hiện tại:
- `notifier.go` hiện tại: `sendEmail` (L128-159) gọi `n.delivery.Send(ctx, notifapp.DeliveryMessage{NotificationID: fmt.Sprintf("adhoc.%s.%s", label, proposalID), ...})` — đúng là điểm bị thay thế
- `service.go` `dispatchNotificationAsync` (L41-55) — đúng là điểm bị xoá goroutine spawn, đúng như Batch 1 completion report đã ghi nhận: "`dispatchNotificationAsync` / `context.Background()` (CF-12, Batch 2 territory) — **not touched**"
- `ADHOC_EMAIL_OUTBOX_ENABLED` — confirmed bằng grep: **chưa tồn tại** trong `config.go` — phải được thêm mới (Must-Create, không phải Must-Update như spec text gợi ý loosely; thực tế là cờ hoàn toàn mới)
- Master Finding Table (AC.2): CF-03/CF-04/CF-12/CF-14 đều map vào Batch 2 — khớp với 2 file trên (`notifier.go` cho CF-03/04, `service.go` cho CF-12)

→ Caller production đầu tiên của `EmailNotificationService` theo spec **chính là** 4 `Notify*` methods của `AdhocProposalNotifier` (`notifier.go`), được gọi qua `NotificationService.DispatchEmail`.

# Shadow Mode Analysis

**Current behavior**: `EMAIL_SHADOW_MODE` được khai báo tại `internal/platform/config/config.go:106` (`EmailShadowMode bool`), đăng ký qua `boolEnv("EMAIL_SHADOW_MODE", false)` (L191). Confirmed bằng grep toàn repo: **đây là tham chiếu DUY NHẤT trong production code** (cộng với `config_test.go:72-73` chỉ kiểm tra giá trị mặc định = false). Cờ hoàn toàn dormant — không có bất kỳ runtime branch, comparison logic, hay metric emission nào tham chiếu đến nó. Trạng thái hiện tại = inert/no-op.

**Desired behavior** (per §AE.2/AE.3): cờ này được "reused" cho 2 mục đích:
- (a) Batch 2A's wiring-proof — đã hoàn thành, không cần `EMAIL_SHADOW_MODE` thật sự active
- (b) **Batch 2's content-migration shadow window** — chạy song song legacy path (`DeliveryAdapter.Send` trực tiếp) và durable path (`NotificationService.DispatchEmail`), so sánh outcome qua từng idempotency key, emit metric `cobo_adhoc_email_shadow_total{outcome="match"|"mismatch"}`

§AE.3 Shadow Mode Rollout Design quy định: **24h STAGING + 48h PROD windows, zero mismatches required, bất kỳ mismatch nào reset đồng hồ** trước khi `ADHOC_EMAIL_OUTBOX_ENABLED` được phép bật thật.

**Gap cần lưu ý**: hiện không có code nào đọc `cfg.EmailShadowMode`. Để Batch 2 vận hành đúng spec, cần thêm logic đọc cờ này vào đúng nơi gating (`notifier.go` hoặc `service.go`) — đây thuộc phạm vi "Must-Change" đã nêu ở §AK.5, không phải một surface mới.

# Cutover Strategy

| | Mô tả |
|---|---|
| **Current Path** | `service.go` 4 call sites → `dispatchNotificationAsync` (spawn goroutine, `context.Background()`) → `AdhocProposalNotifier.Notify*` → `sendEmail` → `DeliveryAdapter.Send` trực tiếp, đồng bộ, lỗi chỉ `log.Warn`, không có outbox/retry |
| **Future Path** | `service.go` 4 call sites → gọi trực tiếp với `ctx` thật (không goroutine) → `AdhocProposalNotifier.Notify*` → `notifier.go` gọi `NotificationService.DispatchEmail(ctx, DispatchEmailRequest{..., IdempotencyKey: "adhoc.<eventType>.<proposalID>.<recipientMembershipID>"})` → durable transactional outbox (`email.dispatch`) → `EmailDispatchHandler` → `DeliveryAdapter.Send` với full retry/backoff/audit trail |
| **Trigger** | Flip `ADHOC_EMAIL_OUTBOX_ENABLED=true`, được entry-gate bởi cửa sổ `EMAIL_SHADOW_MODE` sạch (zero mismatch, đủ 24h STAGING + 48h PROD theo §AE.3) |
| **Fallback** | Legacy `DeliveryAdapter`/templates/renderer/SMTP wiring trong `httpserver/server.go` L440-500 vẫn giữ nguyên ("kept fully intact per §AE.2's retirement ordering") — cả hai path coexist trong giai đoạn shadow |
| **Rollback** | Set `ADHOC_EMAIL_OUTBOX_ENABLED=false` — "instant revert to the legacy goroutine + `DeliveryAdapter.Send` path" |

# File Map

**Must Change** (per §AK.5, verbatim, không suy đoán):
- `internal/adhoc/infra/notification/notifier.go` — thay `n.delivery.Send(...)` bằng `NotificationService.DispatchEmail(ctx, DispatchEmailRequest{..., IdempotencyKey: "adhoc.<eventType>.<proposalID>.<recipientMembershipID>"})` trong `sendEmail`/4 `Notify*` methods, "no wrapper of any kind"
- `internal/adhoc/app/service.go` — xoá `dispatchNotificationAsync`'s `context.Background()` goroutine spawn (L41-55), truyền `ctx` trực tiếp qua 4 call sites (L168, L212, L320, L373); resolves CF-12
- `internal/platform/config/config.go` — thêm field mới `AdhocEmailOutboxEnabled bool` + `boolEnv("ADHOC_EMAIL_OUTBOX_ENABLED", false)` (confirmed: chưa tồn tại, phải tạo mới)

**Must Create**: không có file mới theo spec — handler (`EmailDispatchHandler`), repos (`EmailNotificationRepository`/`EmailDeliveryAttemptRepository`), worker registration cho `email.dispatch` đã tồn tại đầy đủ từ Batch 2A

**Must Not Change**:
- `internal/notification/app/email_service.go`, `email_dispatch_handler.go`, `email_dispatch_contracts.go`, infra repos — Batch 2A đã hoàn thiện wiring, không cần sửa thêm
- Migrations `0051`/`0052` — "Must-Update-Migration: none"
- `internal/adhoc/observability/...` — Batch 5(a) territory
- `internal/iam/...`, `internal/reminder/...` — pipeline khác, ngoài phạm vi Batch 2 (per §AK.5 explicit scoping vào `internal/adhoc/...`)
- Retry/backoff constants (`MaxEmailDeliveryAttempts = 5`, `EmailRetryBackoff`) — định nghĩa sẵn, không đổi

# Risks

1. **Double-send trong Shadow Mode**: nếu logic shadow chạy song song cả legacy `DeliveryAdapter.Send` lẫn durable `DispatchEmail` để so sánh, recipient thật có thể nhận 2 email cho cùng 1 sự kiện trước khi cutover. Spec không nêu rõ cơ chế "compare without double-delivering" — cần làm rõ liệu shadow mode chỉ ghi log/metric phía durable path (không thực sự gửi) hay cả hai path đều gửi thật.
2. **Partial migration**: `notifier.go` có 4 `Notify*` methods × 2 template/method (theo bảng 4 template đã liệt kê) đi qua cùng `sendEmail` helper — thay đổi phải all-or-nothing trong `sendEmail` để tránh trạng thái nửa-cũ-nửa-mới (1 số notification dùng legacy path, 1 số dùng durable path) gây khó trace/audit.
3. **Shadow mismatch / idempotency-key collision**: format `adhoc.<eventType>.<proposalID>.<recipientMembershipID>` phải khớp chính xác để metric `cobo_adhoc_email_shadow_total{outcome}` so sánh đúng cặp; sai lệch format sẽ tạo false-mismatch, reset đồng hồ 24h/48h liên tục.
4. **Rollback gap**: cần verify rằng legacy `DeliveryAdapter`/templates/renderer wiring tại `httpserver/server.go` L440-500 thực sự còn nguyên vẹn và reactivatable tức thì sau khi code thay đổi `notifier.go`/`service.go` — vì `sendEmail` (điểm thay thế) chính là nơi cả 2 path hội tụ, một thay đổi cẩu thả có thể vô tình xoá legacy path thay vì giữ song song.
5. **`EMAIL_SHADOW_MODE` chưa có logic đọc**: cờ hiện hoàn toàn dormant — Batch 2 phải tự thêm logic gating dựa trên nó (chưa tồn tại ở đâu trong runtime), việc này không được nêu rõ là thuộc "Must-Change" file nào — cần làm rõ trước khi code.
6. **`ADHOC_EMAIL_OUTBOX_ENABLED` chưa tồn tại**: spec mô tả nó như "already registered" nhưng grep xác nhận **chưa có** trong `config.go` — đây là điểm spec và thực tế lệch nhau, cần Batch 2 tạo mới hoàn toàn (không phải "update").

# Batch 2 Readiness Verdict

**READY WITH CLARIFICATIONS**

Đã có đủ căn cứ grounded (§AK.5 nêu đích danh 2 file Must-Change + idempotency-key format + flag name; codebase hiện trạng khớp với mô tả của Batch 1 completion report — `dispatchNotificationAsync` "not touched"; Batch 2A đã chứng minh durable pipeline hoạt động end-to-end bằng E2E thực thi PASS). Tuy nhiên còn 3 điểm cần làm rõ trước khi bắt đầu code:

1. **Shadow Mode delivery semantics** — liệu cả hai path (legacy + durable) đều thực sự gửi email tới recipient thật trong cửa sổ shadow, hay chỉ 1 path gửi thật còn path kia chỉ "dry-run" để so sánh? Đây quyết định liệu Risk #1 (double-send) là rủi ro thật hay đã được spec loại trừ bằng thiết kế.
2. **`EMAIL_SHADOW_MODE` gating location** — cờ chưa được tham chiếu ở bất kỳ đâu; cần xác định chính xác logic đọc cờ này nằm trong `notifier.go`, `service.go`, hay một lớp gating mới (và lớp đó có vi phạm "no wrapper of any kind" không).
3. **`ADHOC_EMAIL_OUTBOX_ENABLED` field placement** — spec mô tả "already registered" nhưng thực tế chưa tồn tại; cần xác nhận tên field Go (`AdhocEmailOutboxEnabled` hay tên khác) để tránh xung đột với convention hiện có trong `config.go`.

Không có gap cấu trúc lớn nào — cả 2 file Must-Change, format idempotency key, và đường đi durable pipeline đều đã grounded chính xác từ spec + code thực tế (zero suy đoán). Khuyến nghị làm rõ 3 điểm trên trong một vòng hỏi-đáp ngắn (hoặc đọc thêm phần spec mô tả chi tiết Shadow Mode comparison logic nếu có) trước khi viết contract-first cho Batch 2.
