# Email Notification System — Detailed Implementation Plan

> Updated: 2026-05-22
> Source SA reviewed: `cobo_web_design/docs/email-notification-system-proposal.md`
> Target repo: `cobo_iam_services`
> Audience: backend, frontend/admin, QA, DevOps, Tech Lead, reviewer

## 1. Implementation Principles

- **Incremental migration**
  Không rewrite một lần. Mỗi phase phải để lại hệ thống đang chạy được, có flag, có fallback, có rollback.
- **Backward compatibility**
  Phase 0-3 không được làm thay đổi business behavior của:
  - OTP verification
  - user reset password
  - admin reset password
  - user invitation
  - existing user company invitation
  - reminder deadline email
- **No business behavior change in early phase**
  Source of truth cho token/OTP/TTL vẫn là:
  - `internal/iam/app/service.go`
  - `internal/iam/infra/mysql/auth_recovery.go`
  - `internal/iam/infra/mysql/invitation_mysql.go`
  - `internal/reminder/app/service.go`
- **Feature-flag driven rollout**
  Mọi thay đổi có risk ở path gửi email phải có flag tắt/bật độc lập giữa auth và reminder.
- **Observability before risky rollout**
  Trước khi migrate traffic thật, phải nhìn được:
  - email nào được tạo
  - render có thành công không
  - SMTP có fail không
  - retry đang ở đâu
- **Small PRs, easy rollback**
  Mỗi PR chỉ làm một việc. Không gộp phase 2, 3, 4 vào một PR lớn.
- **Database migration must be backward compatible**
  Migrations tạo bảng/cột mới phải không làm code cũ fail. Code mới phải chạy được cả khi flag đang tắt.
- **Sensitive data must never be logged**
  Không log raw:
  - `OTPCode`
  - `ResetLink`
  - `SetupLink`
  - `RawToken`
  - bất kỳ query param chứa token
- **Existing auth/reminder logic remains source of truth**
  Notification layer chỉ lo:
  - template resolution
  - render
  - persist tracking
  - enqueue outbox
  - delivery
  Nó không tự tính TTL, không tự generate token/OTP, không thay invitation lifecycle.
- **Prefer extend-over-replace for current repo**
  Repo hiện đã có `internal/notification` cho generic notification jobs. Phase đầu nên mở rộng module này theo nhánh `email/` hoặc file mới, không đập bỏ package đang chạy.

## 2. Scope & Non-Scope

### In scope

- thêm email-focused capability vào `internal/notification`
- `TemplateRegistry`
- template source qua `embed.FS`
- `Renderer`
- `NotificationService`
- `SMTPDeliveryAdapter`
- bảng `email_notifications`
- bảng `email_delivery_attempts`
- outbox handler `email.dispatch`
- migrate reminder từ direct SMTP sang `NotificationService`
- hỗ trợ cơ bản `plain text + HTML`
- locale fallback cơ bản
- metrics, logs, alert-ready fields
- CLI/API quản lý template ở phase sau

### Out of scope

- full admin UI cho template ở phase đầu
- full DB-driven template ở phase đầu
- tích hợp external provider như SendGrid/Mailgun nếu SMTP hiện tại còn đáp ứng
- Kafka/RabbitMQ nếu outbox hiện tại còn đủ
- đổi business rule của OTP/reset/invite/reminder
- thay đổi auth API response contract
- thay đổi scheduler semantic của reminder trước khi có test baseline

## 3. Current Code Impact Map

| Area | Current file/package | Current responsibility | Change required | Risk level | Migration approach |
|---|---|---|---|---|---|
| Auth OTP email | `internal/iam/app/service.go` | tạo OTP, build subject/body, publish outbox event `auth.email_verification_requested` | tách render sang template registry/renderer; sau đó migrate sang `NotificationService` | High | phase 0 golden tests -> phase 1 template extraction -> phase 4 canary |
| User password reset email | `internal/iam/app/service.go` | tạo reset token, build reset link, publish `auth.password_reset_requested` | giữ nguyên token logic; chuyển phần email content + dispatch | High | migrate riêng từng email type |
| Admin password reset email | `internal/iam/app/service.go` | admin-triggered reset email | tương tự user reset nhưng idempotency và audit rõ hơn | High | PR riêng sau user reset |
| User invitation email | `internal/iam/app/service.go`, `internal/companyaccess/app/admin_service.go` | build email cho new user và existing user with/without raw token | tách template theo 2 variants rõ ràng | High | giữ raw token flow cũ; chỉ đổi render/delivery từng bước |
| Existing user company invitation email | `internal/iam/app/service.go` | branch `rawToken == ""`, text-only membership-added email | tạo template key riêng, tránh reuse sai với new user invite | Medium | phase 1 template extraction, phase 4 auth migration |
| Reminder disclosure deadline email | `internal/reminder/infra/email/smtp_sender.go` | render hardcoded reminder template + direct SMTP + temporary/permanent classification | thay bằng `NotificationService.DispatchEmail`, giữ occurrence semantics | High | phase 5 only, sau khi auth path ổn |
| Worker outbox processor | `cmd/worker/main.go` | process `notification.dispatch` generic event, process auth email events trực tiếp, helper `deliverAuthEmailEvent` | thêm `email.dispatch` handler mới; dần retire auth-specific handlers | High | dual-path + feature flags |
| SMTP helper | `cmd/worker/main.go`, `internal/reminder/infra/email/smtp_sender.go` | dựng MIME text/plain và gọi `smtp.SendMail` | gom về `SMTPDeliveryAdapter` + `LogOnlyDeliveryAdapter` | Medium | phase 3 |
| Existing notification module | `internal/notification/app/*`, `internal/notification/infra/mysql/*` | generic notification jobs + recipient resolution + outbox event `notification.dispatch` | mở rộng an toàn, không làm vỡ generic notification job flow hiện tại | Medium | add email-focused sub-tree/file set; không đổi contract cũ ở phase đầu |
| Config/env | `internal/platform/config/config.go`, deploy env | SMTP config, TTL config, web base URL | thêm flags, retry config, locale default, template source | Medium | additive env only |
| Tests | `internal/iam/app/service_test.go`, `internal/reminder/infra/email/smtp_sender_test.go`, worker tests | coverage chưa đủ cho golden output và notification lifecycle | bổ sung unit/integration/regression fixtures | High | phase 0 first |

## 4. Target Package Structure

### Recommended structure

Do repo đã có `internal/notification` đang chạy, cấu trúc an toàn nhất là **mở rộng module hiện có bằng nhánh email-focused**, không overwrite package generic hiện tại.

```text
internal/
  notification/
    app/
      contracts.go                     # existing generic notification contracts
      service.go                       # existing generic notification service
      email_contracts.go               # new email NotificationService contracts
      email_service.go                 # new DispatchEmail / PreviewEmail orchestration
      email_renderer.go                # render subject/text/html
      email_sanitizer.go               # redact sensitive vars before persist/log
      email_errors.go                  # transient/permanent/render/validation errors
      email_locale.go                  # locale fallback resolution
    domain/
      notification.go                  # existing generic notification job model
      email_notification.go            # email_notifications entity
      email_delivery_attempt.go        # email_delivery_attempts entity
      email_template.go                # template metadata model
    infra/
      registry/
        embed_registry.go              # embed.FS template source
        db_registry.go                 # phase 7
        hybrid_registry.go             # phase 7
      smtp/
        adapter.go                     # SMTPDeliveryAdapter
        log_adapter.go                 # log-only adapter
        mime.go                        # MIME builder
      mysql/
        repository.go                  # existing generic notification repo
        email_notification_repository.go
        email_template_repository.go   # phase 7
    templates/
      auth.email_verification/
        meta.yaml
        vi/
          subject.txt
          body.txt
          body.html
      auth.password_reset.user/
      auth.password_reset.admin/
      auth.user_invitation.new_user/
      auth.user_invitation.existing_user/
      reminder.disclosure_deadline/

cmd/
  worker/
    main.go
    email_dispatch_handler.go

cmd/
  tools/
    email_preview.go
    email_validate.go
    email_publish.go                   # phase 7
    email_rollback.go                  # phase 7
```

### Package/file responsibilities

#### `internal/notification/app/email_contracts.go`

- **Responsibility**
  định nghĩa `DispatchEmail`, `PreviewEmail`, request/response, enum lỗi
- **Called by**
  `iam.Service`, reminder pipeline, admin tools
- **Must not depend on**
  SMTP, `net/smtp`, transport layer
- **Tests**
  request validation, defaulting, idempotency field validation

#### `internal/notification/app/email_service.go`

- **Responsibility**
  orchestrate:
  - validate request
  - resolve template
  - sanitize vars
  - persist notification
  - publish outbox
- **Called by**
  auth flow, reminder flow, CLI preview/publish indirectly
- **Must not depend on**
  direct SMTP
- **Tests**
  shadow mode, dual-write, duplicate idempotency key, transactional enqueue

#### `internal/notification/app/email_renderer.go`

- **Responsibility**
  render `subject.txt`, `body.txt`, `body.html`
- **Called by**
  `NotificationService`, worker preview/send path
- **Must not depend on**
  DB, SMTP
- **Tests**
  missing vars, escaping, golden outputs, locale fallback

#### `internal/notification/app/email_sanitizer.go`

- **Responsibility**
  mask raw sensitive vars trước khi:
  - persist vào DB
  - log
  - emit structured event
- **Called by**
  service, worker
- **Must not depend on**
  transport or SMTP
- **Tests**
  redaction matrix for OTP/reset/setup/raw token

#### `internal/notification/infra/registry/embed_registry.go`

- **Responsibility**
  load template metadata + locale files từ `embed.FS`
- **Called by**
  renderer/service
- **Must not depend on**
  MySQL
- **Tests**
  missing file, locale fallback, malformed meta

#### `internal/notification/infra/registry/db_registry.go`

- **Responsibility**
  phase 7 runtime override from DB
- **Called by**
  hybrid registry
- **Must not depend on**
  SMTP
- **Tests**
  active version lookup, bad draft rejection

#### `internal/notification/infra/smtp/adapter.go`

- **Responsibility**
  send rendered email via SMTP, return normalized result
- **Called by**
  `email_dispatch_handler.go`
- **Must not depend on**
  business modules
- **Tests**
  transient vs permanent classification, UTF-8 subject, multipart body

#### `internal/notification/infra/smtp/log_adapter.go`

- **Responsibility**
  log-only mode when `SMTP_HOST` empty
- **Called by**
  worker in dev/test/staging fallback
- **Must not depend on**
  DB
- **Tests**
  no body/token leak, accepted result contract

#### `internal/notification/infra/smtp/mime.go`

- **Responsibility**
  build MIME:
  - `text/plain`
  - `text/html`
  - multipart/alternative
  - `Message-ID`
  - UTF-8 encoded subject
- **Tests**
  raw MIME snapshots for Gmail/Outlook-safe structure

#### `internal/notification/infra/mysql/email_notification_repository.go`

- **Responsibility**
  CRUD cho `email_notifications`, update status, idempotency lookup
- **Called by**
  `NotificationService`, worker
- **Must not depend on**
  SMTP
- **Tests**
  insert/update/unique conflict/index coverage

#### `internal/notification/infra/mysql/email_template_repository.go`

- **Responsibility**
  phase 7 template DB storage
- **Tests**
  version publish/rollback/audit trail

#### `cmd/worker/email_dispatch_handler.go`

- **Responsibility**
  xử lý outbox event `email.dispatch`
- **Called by**
  outbox processor
- **Must not depend on**
  auth business logic
- **Tests**
  idempotent send, retry path, status transitions

#### `cmd/tools/email_preview.go`

- **Responsibility**
  preview template với sample vars mà không gửi
- **Called by**
  developer, operator, QA
- **Tests**
  CLI input validation, output formatting

#### `cmd/tools/email_validate.go`

- **Responsibility**
  validate template completeness trước publish
- **Tests**
  missing locale/body/meta/schema mismatch

## 5. Phase-by-Phase Implementation Plan

## Phase 0 — Preparation & Safety Baseline

### Mục tiêu

- khóa baseline behavior trước khi refactor
- có test và fixture đủ để biết output đang đổi hay chưa
- có env/flag map rõ ràng

### Backend tasks

1. Audit toàn bộ nơi gửi email hiện tại.
2. Chốt 6 luồng email baseline:
   - OTP verification
   - user password reset
   - admin password reset
   - new user invitation
   - existing user invitation
   - reminder deadline email
3. Snapshot subject/body hiện tại thành golden fixtures.
4. Viết regression tests cho 6 luồng trên.
5. Thêm feature flags:
   - `EMAIL_NOTIFICATION_ENABLED`
   - `EMAIL_TEMPLATE_SOURCE`
   - `EMAIL_DELIVERY_PATH`
   - `EMAIL_FORMAT`
   - `EMAIL_SHADOW_MODE`
6. Document env hiện tại và mapping env mới.

### Proposed PRs

- `PR-01`: audit + fixtures + golden tests
- `PR-02`: config/env flags + docs

### Files touched

- `internal/iam/app/service_test.go`
- `internal/reminder/infra/email/smtp_sender_test.go`
- `cmd/worker/main.go` tests if needed
- `internal/platform/config/config.go`
- `docs/email-notification-system-implementation-plan.md`
- new fixtures under `testdata/email-golden/`

### Test cases

- subject/body của từng email match baseline
- OTP/reset/invite/reminder APIs không đổi response
- `SMTP_HOST=""` vẫn chạy log-only

### Rollback

- code additive only
- disable flags by env
- remove test-only fixtures nếu cần, không ảnh hưởng runtime

### Completion criteria

- 6 golden tests pass
- env flags available but default safe
- reviewer có thể chỉ ra chính xác current behavior từ fixture

## Phase 1 — Extract Templates to embed.FS Without Behavior Change

### Mục tiêu

- tách subject/body khỏi service code
- output email **phải giống hiện tại**
- delivery path cũ vẫn giữ nguyên

### Implementation details

1. Tạo `internal/notification/templates`.
2. Convert 6 templates hiện tại sang file:
   - `subject.txt`
   - `body.txt`
   - `meta.yaml`
3. Chưa bắt buộc HTML cho tất cả template.
4. Tạo `EmbedRegistry`.
5. Tạo `Renderer` dùng `text/template`.
6. Tạo variable schema trong `meta.yaml`.
7. Update auth/reminder code:
   - render từ template mới
   - vẫn publish/send qua path cũ
8. Nếu render lỗi:
   - fallback về hardcoded legacy path
   - gated bằng `EMAIL_TEMPLATE_SOURCE=legacy`

### Backend task breakdown

- add `embed_registry.go`
- add `email_renderer.go`
- add template fixtures
- wire auth template render behind `EMAIL_TEMPLATE_SOURCE=embed`
- wire reminder render behind same flag but still direct SMTP

### Code skeleton

```go
type TemplateRegistry interface {
    Resolve(ctx context.Context, key, locale string) (ResolvedTemplate, error)
}

type Renderer interface {
    Render(t ResolvedTemplate, vars map[string]any) (RenderedEmail, error)
}

type RenderedEmail struct {
    Subject string
    TextBody string
    HTMLBody string
}
```

### Migration

- không cần DB migration

### Test checklist

- registry load success/fail
- renderer missing var
- renderer output equals golden text
- fallback legacy path when registry/render fails

### Rollback plan

- set `EMAIL_TEMPLATE_SOURCE=legacy`
- leave template files in tree; no DB/state cleanup needed

### Risk mitigation

- không đổi outbox payload
- không đổi reminder retry
- không đổi token generation / TTL

### Completion criteria

- 6 email outputs match golden tests under `embed` mode
- runtime can switch back to `legacy` without code revert

## Phase 2 — Introduce NotificationService Interface

### Mục tiêu

- tạo single entry point cho email dispatch
- chưa chuyển traffic thật toàn phần
- có shadow mode để so sánh

### Implementation details

1. Tạo `NotificationService`:
   - `DispatchEmail(ctx, req)`
   - `PreviewEmail(ctx, key, locale, vars)`
2. Tạo `DispatchEmailRequest`:
   - `To`
   - `TemplateKey`
   - `Locale`
   - `Variables`
   - `IdempotencyKey`
   - `TriggeredBy`
   - `CompanyID`
   - `ScheduledAt`
   - `SourceEventType`
3. Tạo bảng `email_notifications`.
4. Implement repository.
5. `DispatchEmail` thực hiện:
   - validate request
   - resolve template
   - render validation
   - sanitize variables
   - insert `email_notifications`
   - publish outbox event `email.dispatch`
6. Shadow mode:
   - legacy path vẫn là source of truth
   - notification service chỉ render + persist + outbox shadow, hoặc chỉ persist tùy flag
   - nếu shadow insert fail thì **không fail business transaction**

### Recommended table shape

`email_notifications`

- `email_notification_id`
- `company_id`
- `recipient_email`
- `template_key`
- `locale`
- `status` (`pending`, `sending`, `sent`, `retry`, `failed_permanent`, `cancelled`)
- `idempotency_key`
- `triggered_by_user_id`
- `source_event_type`
- `source_aggregate_type`
- `source_aggregate_id`
- `variables_json_sanitized`
- `scheduled_at`
- `last_error_code`
- `last_error_message_redacted`
- `sent_at`
- `created_at`
- `updated_at`

### Repository methods

- `CreateNotification`
- `GetByID`
- `GetByIdempotencyKey`
- `MarkSending`
- `MarkSent`
- `MarkRetry`
- `MarkFailedPermanent`

### Error handling

- validation/render/repository errors classified riêng
- shadow mode swallows persistence failure after structured log + metric increment

### SQL migration

- add table only
- unique index: `(idempotency_key)`
- supporting indexes:
  - `(status, scheduled_at, created_at)`
  - `(template_key, created_at)`
  - `(company_id, created_at)`

### Shadow mode strategy

- `EMAIL_SHADOW_MODE=true`
- `EMAIL_DELIVERY_PATH=legacy`
- create notification record + optional outbox shadow event but worker ignores if `EMAIL_NOTIFICATION_ENABLED=false`

### Tests

- request validation
- duplicate idempotency key -> no duplicate record
- sanitized vars persisted
- shadow mode failure does not break auth/reminder path
- transactional enqueue when MySQL outbox available

### Rollback

- disable `EMAIL_NOTIFICATION_ENABLED`
- keep table; code old path still works

### Completion criteria

- `NotificationService` available and covered by tests
- shadow records created for selected flows
- no production behavior change when flags remain safe

## Phase 3 — Add Email Dispatch Worker & SMTPDeliveryAdapter

### Mục tiêu

- worker xử lý `email.dispatch`
- có delivery adapter chuẩn hóa
- có retry tracking riêng

### Implementation details

1. Tạo `SMTPDeliveryAdapter`.
2. Tạo `LogOnlyDeliveryAdapter` khi `SMTP_HOST` rỗng.
3. Tạo `mime.go` hỗ trợ:
   - `text/plain`
   - `text/html`
   - UTF-8 subject
   - `Message-ID`
4. Tạo outbox handler `email.dispatch`.
5. Handler flow:
   1. load notification by ID
   2. skip nếu `status=sent`
   3. `MarkSending`
   4. resolve template / or reuse persisted rendered snapshot if later chosen
   5. render
   6. send via adapter
   7. insert delivery attempt
   8. update notification status
6. Tạo bảng `email_delivery_attempts`.
7. Chuẩn hóa retry:
   - `max_attempts`
   - exponential backoff with cap
   - transient/permanent classification
8. Chưa chuyển reminder sang path mới.

### Recommended `email_delivery_attempts`

- `email_delivery_attempt_id`
- `email_notification_id`
- `attempt_no`
- `provider` (`smtp`, `log_only`)
- `status` (`sending`, `sent`, `retry`, `failed_permanent`)
- `smtp_response_code` nullable
- `error_code`
- `error_message_redacted`
- `started_at`
- `finished_at`
- `next_retry_at`
- `created_at`

### Worker pseudo-code

```go
func HandleEmailDispatch(ctx context.Context, event outbox.QueuedEvent) error {
    notifID := payload.NotificationID
    n, err := repo.GetByID(ctx, notifID)
    if err != nil { return err }
    if n.Status == "sent" { return nil }

    if err := repo.MarkSending(ctx, notifID); err != nil { return err }

    rendered, err := renderer.Render(...)
    if err != nil {
        repo.InsertAttempt(...failed_render...)
        return repo.MarkFailedPermanent(ctx, notifID, "render_error", redact(err))
    }

    result, err := adapter.Send(ctx, SendInput{...})
    if err == nil {
        repo.InsertAttempt(...sent...)
        return repo.MarkSent(ctx, notifID, result.ProviderMessageID)
    }

    class := classify(err)
    if class == transient && attempts < maxAttempts {
        repo.InsertAttempt(...retry...)
        return repo.MarkRetry(ctx, notifID, nextBackoff(...), class.Code, redact(err))
    }

    repo.InsertAttempt(...failed...)
    return repo.MarkFailedPermanent(ctx, notifID, class.Code, redact(err))
}
```

### Retry design

- default `max_attempts = 5`
- backoff: `1m, 5m, 15m, 1h, 6h`
- transient:
  - network timeout
  - 421/450/451/452
  - DNS temporary
  - SMTP auth temp if provider marks retriable
- permanent:
  - invalid recipient format
  - render error
  - unsupported template key
  - 550/551/553 style hard failures

### Tests

- fake adapter success
- transient error -> retry scheduled
- permanent error -> failed_permanent
- duplicate outbox redelivery -> no second send after `sent`
- log adapter hides body/token

### Rollback

- stop registering `email.dispatch`
- set `EMAIL_NOTIFICATION_ENABLED=false`
- keep tables for inspection

### Completion criteria

- worker can process test notification end-to-end
- attempts table populated correctly
- no raw body/token in worker logs

## Phase 4 — Migrate Auth Emails to NotificationService

### Mục tiêu

- auth emails đi qua `NotificationService`
- rollout nhỏ, có canary, migrate từng email type

### Migration order

1. OTP verification
2. user password reset
3. admin password reset
4. new user invitation
5. existing user invitation

### Mapping table

| Old event / path | New template key | Variables | Sensitive vars | Idempotency key | Expected subject | Test cases |
|---|---|---|---|---|---|---|
| `auth.email_verification_requested` | `auth.email_verification` | `FullName`, `LoginID`, `OTPCode`, `ExpiryMinutes` | `OTPCode` | `auth:email_verification:{user_id}:{otp_id or code_hash}` | `Verify your email` | render golden, duplicate resend behavior, no API change |
| `auth.password_reset_requested` | `auth.password_reset.user` | `FullName`, `LoginID`, `ResetLink`, `ExpiryMinutes` | `ResetLink`, `RawToken` | `auth:password_reset:user:{user_id}:{token_hash}` | `Reset your password` | generic success response preserved |
| `auth.admin_password_reset_requested` | `auth.password_reset.admin` | `FullName`, `LoginID`, `ResetLink`, `ExpiryMinutes` | `ResetLink`, `RawToken` | `auth:password_reset:admin:{user_id}:{token_hash}` | `Dat lai mat khau (yeu cau tu quan tri)` | admin action audit intact |
| `auth.user_invitation_sent` with token | `auth.user_invitation.new_user` | `DisplayName`, `CompanyName`, `SetupLink`, `ExpiryHours` | `SetupLink`, `RawToken` | `auth:user_invitation:new:{user_id}:{invitation_id}` | `Thiet lap mat khau tai khoan` | invitation accept flow unaffected |
| `auth.user_invitation_sent` without token | `auth.user_invitation.existing_user` | `DisplayName`, `CompanyName` | none | `auth:user_invitation:existing:{user_id}:{company_id}:{membership_id or invitation_id}` | `Tham gia cong ty` | no reset/setup link present |

### Implementation details

1. Giữ nguyên nơi tạo token/OTP.
2. Thay `publishEmail(...)` bằng helper mới:
   - build `DispatchEmailRequest`
   - if flag off -> old publish
   - if shadow -> both
   - if on -> notification service only
3. Mỗi email type có flag riêng hoặc rollout switch riêng trong config helper.
4. Không publish đồng thời legacy outbox và `email.dispatch` trong mode production rollout cuối cùng.

### Canary rollout

- staging full
- internal test users only
- production:
  - enable OTP first
  - monitor 24h
  - then reset user
  - then admin reset
  - then invitations

### Regression plan

- compare staged subject/body against golden
- compare API response snapshots
- test invitation accept and reset token consumption

### Rollback

- per-email-type flag back to legacy
- leave notification rows for audit; no data revert needed

### Completion criteria

- auth emails fully on `NotificationService`
- legacy auth-specific outbox handlers no longer needed for migrated types
- no increase in support incidents for missing/duplicate email

## Phase 5 — Migrate Reminder Emails from Direct SMTP to NotificationService

### Mục tiêu

- loại bỏ direct SMTP trong reminder
- không mất reminder, không double retry

### Current reminder flow to preserve

- scheduler chọn occurrence đến hạn
- occurrence được claim/process
- `smtp_sender.go` render + send trực tiếp
- reminder repository cập nhật send state

### Migration design

1. Thay `smtp_sender.go` send path bằng `NotificationService.DispatchEmail`.
2. Dùng occurrence id làm idempotency root:
   - `reminder:disclosure_deadline:{occurrence_id}`
3. Không để reminder layer retry SMTP riêng nữa khi `EMAIL_REMINDER_USE_NOTIFICATION_SERVICE=true`.
4. Reminder layer chỉ retry phần tạo dispatch nếu chưa enqueue được.
5. Delivery retry sau enqueue thuộc về email delivery worker.

### Sequence diagram text

1. reminder scheduler claim occurrence
2. build reminder business payload
3. call `DispatchEmail(...)`
4. if dispatch create success:
   - mark occurrence `queued_for_delivery` or current equivalent
5. worker handles `email.dispatch`
6. worker updates `email_notifications` + `email_delivery_attempts`
7. optional callback/reconciliation updates occurrence final status if domain requires

### State transition proposal

- `pending` -> `claimed`
- `claimed` -> `email_queued`
- `email_queued` -> `delivered`
- `email_queued` -> `delivery_retrying`
- `email_queued` -> `delivery_failed_permanent`

If current schema cannot support new states safely in one sprint:
- keep existing reminder occurrence states
- add mapping field/notes in code only
- use notification status as authoritative delivery state

### Key decision

- **Không double retry**
  Khi `NotificationService` đã nhận job thành công, reminder service không tự retry SMTP nữa.

### Code-level task list

- add reminder-to-notification adapter
- deprecate direct call to `internal/reminder/infra/email/smtp_sender.go`
- preserve old sender behind flag
- add reconciliation helper if occurrence status needs sync

### Tests

- same occurrence dispatch twice -> one email notification
- enqueue fail before outbox -> reminder remains retryable
- send fail after enqueue -> reminder not duplicated, delivery attempt tracked

### Rollback

- `EMAIL_REMINDER_USE_NOTIFICATION_SERVICE=false`
- switch reminder back to old sender

### Completion criteria

- reminder path no longer sends direct SMTP when flag on
- no duplicate reminder mail for same occurrence
- reminder scheduling semantics unchanged

## Phase 6 — HTML Templates + Basic i18n

### Mục tiêu

- hỗ trợ HTML email
- locale `vi/en` với fallback chain

### Implementation details

1. Thêm `body.html` cho từng template.
2. Giữ `body.txt` fallback.
3. Renderer:
   - `text/template` cho subject/text
   - `html/template` cho HTML
4. Locale resolution chain:
   1. explicit request locale
   2. user preferred locale
   3. company default locale
   4. system default
   5. hard fallback `vi`
5. Thêm `EMAIL_FORMAT=text|html`.
6. Validate action URLs:
   - must be HTTPS outside dev
   - no inline script

### Template checklist

- subject `vi`
- subject `en`
- body.txt `vi`
- body.txt `en`
- body.html `vi`
- body.html `en`
- meta variable schema

### HTML compatibility checklist

- table-based basic layout if branding added
- inline CSS only where needed
- no JS
- escape all user content
- links absolute, not relative

### Tests

- locale fallback
- HTML escaping
- plain text fallback when HTML missing
- MIME multipart generation

### Rollback

- `EMAIL_FORMAT=text`
- locale default back to `vi`

### Completion criteria

- selected templates verified on staging inboxes
- text fallback still works
- no malformed HTML causing send failures

## Phase 7 — DB Override / Admin CLI / Optional Admin UI

### Mục tiêu

- runtime template management nếu cần
- vẫn giữ `embed.FS` as safe fallback

### Implementation details

1. Tạo schema:
   - `email_templates`
   - `email_template_versions`
   - `email_template_locales`
2. Implement `HybridRegistry`:
   - DB active version if enabled and valid
   - fallback `embed.FS`
3. CLI:
   - `email-preview`
   - `email-validate`
   - `email-publish`
   - `email-rollback`
4. Optional API/Admin UI:
   - list templates
   - create draft
   - preview
   - validate
   - publish
   - rollback
5. Permission model:
   - `notification.template.view`
   - `notification.template.edit`
   - `notification.template.publish`
   - `notification.template.rollback`

### Validation rules

- publish must do sample render
- no hard delete version
- bad DB template must not block fallback if override flag off

### Backend tasks

- migrations for template tables
- repositories
- registry fallback policy
- CLI commands
- optional HTTP handlers

### Frontend tasks

- optional CMS/Admin screen only after API stable
- draft editor + preview + publish/rollback

### QA checklist

- publish valid draft
- publish invalid draft rejected
- rollback to prior version
- disable DB override returns to embed templates

### Rollback

- `EMAIL_DB_TEMPLATE_OVERRIDE_ENABLED=false`
- active DB versions remain stored but ignored

### Completion criteria

- runtime override works without code deploy
- embed fallback proven in staging

## Phase 8 — Observability, Audit & Operational Readiness

### Mục tiêu

- production-ready visibility
- clear runbook for support/oncall

### Implementation details

1. Structured logs:
   - notification created
   - render success/fail
   - delivery attempted
   - delivery success
   - delivery failed
2. Metrics:
   - `email_notifications_created_total`
   - `email_delivery_attempts_total`
   - `email_notifications_failed_total`
   - `email_render_duration_seconds`
   - `email_delivery_duration_seconds`
   - `email_outbox_queue_lag_seconds`
   - `email_notifications_pending_total`
3. Dashboard:
   - volume
   - success rate
   - failure rate
   - queue lag
   - retry backlog
   - recent failures by template
4. Alerts:
   - SMTP auth failure spike
   - failure rate > threshold
   - queue lag > threshold
   - render error spike
5. Runbook:
   - user báo không nhận email
   - SMTP down
   - template render fail
   - duplicate email trace by idempotency key

### Completion criteria

- dashboard exists
- alerts wired
- runbook reviewed by backend + DevOps + QA

## 6. Detailed Task Breakdown

### Backend tasks

| Task ID | Description | Package/File | Dependencies | Estimate | Risk | Acceptance Criteria |
|---|---|---|---|---|---|---|
| BE-01 | Audit all email send paths + create golden fixtures | `internal/iam/app`, `internal/reminder/infra/email`, `testdata/email-golden` | none | 2d | Low | 6 golden fixtures committed |
| BE-02 | Add feature flags/env parsing | `internal/platform/config/config.go` | BE-01 | 0.5d | Low | flags available with safe defaults |
| BE-03 | Add embed template tree + registry | `internal/notification/templates`, `infra/registry/embed_registry.go` | BE-01 | 2d | Medium | registry loads 6 templates |
| BE-04 | Add renderer + sanitizer + meta validation | `internal/notification/app/email_*` | BE-03 | 2d | Medium | rendered text matches golden |
| BE-05 | Wire auth/reminder render under template source flag | `internal/iam/app/service.go`, `internal/reminder/infra/email/smtp_sender.go` | BE-04 | 2d | Medium | legacy behavior unchanged by default |
| BE-06 | Add `email_notifications` migration + repo | `migrations/*`, `infra/mysql/email_notification_repository.go` | BE-02 | 2d | Medium | table created, unique idempotency works |
| BE-07 | Add `NotificationService` shadow mode | `internal/notification/app/email_service.go` | BE-06 | 2d | High | shadow insert does not break business path |
| BE-08 | Add `email_delivery_attempts` migration + repo | `migrations/*`, `infra/mysql/*` | BE-06 | 1.5d | Medium | attempts persisted |
| BE-09 | Add SMTP adapter + MIME builder + log adapter | `infra/smtp/*` | BE-08 | 2d | Medium | fake adapter + MIME tests pass |
| BE-10 | Add outbox `email.dispatch` worker handler | `cmd/worker/email_dispatch_handler.go`, `cmd/worker/main.go` | BE-09 | 2d | High | end-to-end dispatch works in integration tests |
| BE-11 | Migrate OTP email | `internal/iam/app/service.go` | BE-10 | 1d | High | OTP path uses service behind flag |
| BE-12 | Migrate user/admin reset emails | `internal/iam/app/service.go` | BE-11 | 1.5d | High | reset flows unchanged |
| BE-13 | Migrate invitation emails | `internal/iam/app/service.go`, `internal/companyaccess/app/admin_service.go` | BE-12 | 2d | High | both invite variants covered |
| BE-14 | Migrate reminder email | `internal/reminder/app/service.go`, `internal/reminder/infra/email/smtp_sender.go` | BE-10 | 2d | High | no direct SMTP when flag on |
| BE-15 | Add HTML + locale fallback | `templates/*`, `email_locale.go`, `mime.go` | BE-14 | 2d | Medium | vi/en + text fallback verified |
| BE-16 | Add DB override + CLI | `infra/registry/db_registry.go`, `cmd/tools/*` | BE-15 | 3d | Medium | preview/publish/rollback commands work |
| BE-17 | Add metrics/logging/runbook hooks | `app/email_service.go`, `cmd/worker/*` | BE-10 | 2d | Medium | metrics exported, logs redacted |

### Frontend/Admin tasks

| Screen/API | Description | Dependencies | Acceptance Criteria |
|---|---|---|---|
| Template admin API contract | define list/draft/preview/publish/rollback endpoints | phase 7 backend API | reviewed and versioned |
| Optional CMS screen: template list | list template keys and active version | phase 7 API | load, filter, view details |
| Optional CMS screen: template editor | edit locale subject/text/html draft | template API + permission model | save draft without publish |
| Optional CMS screen: preview | render sample vars before publish | preview API | shows text/html preview |
| Optional CMS screen: publish/rollback | publish validated draft or rollback active version | publish API | action audited and permission-gated |

### DevOps tasks

| Area | Task | Acceptance Criteria |
|---|---|---|
| Env vars | add flags and retry config to staging/prod env templates | env docs and deployment manifests updated |
| Migration deployment | deploy DB migrations before feature enablement | migrations applied with zero downtime |
| Dashboards | add notification metrics panels | dashboard accessible in staging/prod |
| Alerts | SMTP failure, queue lag, render error alerts | alerts tested with synthetic failure |
| Secrets | verify SMTP creds rotation path | documented and tested in staging |
| Rollback | runbook for flag rollback and worker disablement | oncall can execute without code revert |

### QA tasks

| Test scenario | Test data | Expected result | Regression area | Automation/manual |
|---|---|---|---|---|
| OTP email baseline | seeded user + OTP | subject/body unchanged | register/resend verify | auto |
| User reset password | user with email | generic API response unchanged, email content correct | forgot/reset flow | auto |
| Admin reset password | admin-triggered user | email created once, link valid | admin users flow | auto |
| New user invitation | invited no-account user | setup link email correct | invite/accept flow | auto |
| Existing user invitation | existing active user new company membership | no setup link, correct company copy | add-member flow | auto |
| Reminder deadline email | seeded occurrence due | exactly one reminder per occurrence | reminder dispatch | auto |
| SMTP log-only mode | `SMTP_HOST=""` | no send attempt to real SMTP, safe logs | worker behavior | manual + auto |
| HTML rendering | staging inbox on Gmail/Outlook | layout readable, links work | template rendering | manual |
| Sensitive log leak | trigger all email types | no token/OTP in logs/db sanitized fields | security | auto |

## 7. Database Migration Plan

### Migration order

1. add `email_notifications`
2. add indexes + unique idempotency key
3. add `email_delivery_attempts`
4. add optional extra status columns only if needed
5. phase 7 add template DB tables

### Backward compatibility

- only create new tables first
- do not alter current auth/reminder/outbox tables in early phases
- new code must tolerate table absent only before rollout; migration should ship before enabling feature flags

### Suggested SQL strategy

#### `email_notifications`

- nullable:
  - `company_id`
  - `triggered_by_user_id`
  - `scheduled_at`
  - `last_error_code`
  - `last_error_message_redacted`
  - `sent_at`
- required:
  - `email_notification_id`
  - `recipient_email`
  - `template_key`
  - `locale`
  - `status`
  - `idempotency_key`
  - `variables_json_sanitized`
  - `created_at`
  - `updated_at`

#### `email_delivery_attempts`

- FK to `email_notifications.email_notification_id`
- unique `(email_notification_id, attempt_no)`

### Index strategy

- `email_notifications`
  - unique `(idempotency_key)`
  - index `(status, scheduled_at, created_at)`
  - index `(template_key, created_at)`
  - index `(company_id, created_at)`
- `email_delivery_attempts`
  - index `(email_notification_id, created_at)`
  - index `(status, created_at)`

### Rollback SQL

- application rollback should prefer **flag rollback**, not immediate table drop
- hard SQL rollback only if migration itself failed before feature enablement
- do not drop populated audit tables casually in prod

### Data retention/archive policy

- `email_notifications`
  - hot retention 90-180 days
  - archive or summarize older records
- `email_delivery_attempts`
  - hot retention 30-90 days
  - keep aggregate metrics longer elsewhere

### Deploy order

- migrate DB first
- deploy code second
- enable flags third

Never:
- enable code path requiring table before migration lands

## 8. Feature Flags & Rollout Strategy

| Flag | Default | Phase | When to enable | When to remove | Rollback behavior |
|---|---|---|---|---|---|
| `EMAIL_TEMPLATE_SOURCE=legacy|embed|hybrid` | `legacy` | 1, 7 | enable `embed` in staging after golden tests; `hybrid` only phase 7 | after DB override stable and legacy removed | switch back to `legacy` or `embed` |
| `EMAIL_DELIVERY_PATH=legacy|notification_service` | `legacy` | 2-5 | enable per flow after shadow verification | when all legacy send paths removed | switch to `legacy` |
| `EMAIL_AUTH_EMAILS_USE_NOTIFICATION_SERVICE` | `false` | 4 | enable per auth email batch in staging then prod | when auth fully migrated | set `false` to revert auth to old path |
| `EMAIL_REMINDER_USE_NOTIFICATION_SERVICE` | `false` | 5 | only after auth path stable | when reminder fully migrated | set `false` |
| `EMAIL_FORMAT=text|html` | `text` | 6 | after HTML QA in staging | when HTML becomes universal default | switch to `text` |
| `EMAIL_DB_TEMPLATE_OVERRIDE_ENABLED` | `false` | 7 | only after HybridRegistry and admin tooling stable | maybe never remove if useful | set `false`, fallback embed |
| `EMAIL_SHADOW_MODE` | `false` | 2-4 | enable in staging first, optionally prod for low-risk shadow | remove after full cutover | set `false` to stop shadow writes |
| `EMAIL_NOTIFICATION_ENABLED` | `false` | 2-3 | enable worker processing only after repo/tables/adapter ready | after full migration flags simplified | set `false` to stop new path |

## 9. Testing Strategy

### Unit tests

- `TemplateRegistry`
- `Renderer`
- `sanitizeVars`
- validate required variables
- locale fallback
- SMTP MIME builder
- error classification
- idempotency handling

### Integration tests

- `DispatchEmail` creates notification
- outbox event created
- worker sends via fake SMTP adapter
- retry on transient error
- permanent failure marks failed
- duplicate idempotency key does not create second email

### Regression tests

- compare output 6 email hiện tại với golden files
- auth flow không đổi
- reminder scheduling không đổi
- existing API response không đổi

### E2E tests

- register user -> receive OTP email
- password reset user
- admin password reset
- invite new user
- add existing user to company
- disclosure deadline reminder

### Security tests

- OTP/token không xuất hiện trong log
- `variables_json_sanitized` không chứa raw sensitive vars
- HTML escaping hoạt động
- invalid URL bị reject hoặc fallback

## 10. Risk Register

| Risk ID | Description | Impact | Likelihood | Affected phase | Mitigation | Rollback |
|---|---|---|---|---|---|---|
| R-01 | Duplicate email do dual path hoặc outbox redelivery | High | Medium | 2-5 | idempotency key unique, skip if status sent, no dual-send in full mode | revert flag to legacy |
| R-02 | Missing email do dispatch persist fail | High | Medium | 2-5 | shadow mode swallow only before cutover; transactional enqueue in active mode | revert to legacy send path |
| R-03 | Bad template render | High | Medium | 1-7 | golden tests, preview, meta validation, fallback legacy/embed | switch template source back |
| R-04 | DB migration failure | High | Low | 2,3,7 | additive migrations, stage first, migrate before enable | stop rollout, keep flags off |
| R-05 | SMTP outage | High | Medium | 3-8 | retry policy, log-only in lower env, alerts | queue retries, disable feature if needed |
| R-06 | Sensitive data leak | Critical | Medium | all | sanitizer, redacted logs, security tests | disable new path, rotate impacted tokens if needed |
| R-07 | Reminder double retry | High | Medium | 5 | single owner for delivery retry, dedup on occurrence id | disable reminder notification service flag |
| R-08 | Locale fallback sai | Medium | Medium | 6-7 | explicit fallback tests, default `vi` | revert `EMAIL_FORMAT=text` or force locale |
| R-09 | HTML email hiển thị lỗi | Medium | Medium | 6 | manual inbox QA, plain text fallback | switch `EMAIL_FORMAT=text` |
| R-10 | Business logic bị thay đổi ngoài ý muốn | Critical | Medium | all | phase 0 golden/regression first, keep token logic untouched | immediate flag rollback |

## 11. PR Plan

| PR | Goal | Files touched | Review focus | Test required | Rollback |
|---|---|---|---|---|---|
| PR-01 | Add golden tests and baseline fixtures | `service_test.go`, `smtp_sender_test.go`, `testdata/email-golden/*` | baseline correctness | unit/regression | revert additive tests only |
| PR-02 | Add config flags/env docs | `config.go`, docs | safe defaults | config tests | env back to defaults |
| PR-03 | Add template files + embed registry | `internal/notification/templates`, `infra/registry` | file layout, locale path | registry unit tests | `EMAIL_TEMPLATE_SOURCE=legacy` |
| PR-04 | Add renderer + template validation + sanitizer | `app/email_*` | rendering correctness, redaction | golden tests | keep unused code |
| PR-05 | Wire auth/reminder render behind flag | `iam/app/service.go`, `reminder/infra/email/smtp_sender.go` | no behavior change | regression suite | flip template source to legacy |
| PR-06 | Add `email_notifications` migration + repo | `migrations/*`, `infra/mysql/*` | schema/index/idempotency | integration | leave table unused |
| PR-07 | Add NotificationService shadow mode | `app/email_service.go` | non-breaking shadow semantics | integration | `EMAIL_SHADOW_MODE=false` |
| PR-08 | Add `email_delivery_attempts` + SMTP adapter | `migrations/*`, `infra/smtp/*` | retry classification, no leaks | unit/integration | keep handler off |
| PR-09 | Add outbox `email.dispatch` handler | `cmd/worker/*` | idempotent send | worker integration | `EMAIL_NOTIFICATION_ENABLED=false` |
| PR-10 | Migrate OTP email | `iam/app/service.go` | no API/TTL change | regression/e2e | auth flag false |
| PR-11 | Migrate password reset emails | `iam/app/service.go` | token link safety | regression/e2e | auth flag false |
| PR-12 | Migrate invitation emails | `iam/app/service.go`, `companyaccess/app/*` | new vs existing invite split | regression/e2e | auth flag false |
| PR-13 | Migrate reminder email | `reminder/*` | dedup + retry ownership | regression/e2e | reminder flag false |
| PR-14 | Add HTML templates | `templates/*`, `mime.go` | rendering + compatibility | manual inbox QA | `EMAIL_FORMAT=text` |
| PR-15 | Add i18n fallback | `email_locale.go`, templates | fallback correctness | unit/manual | force default locale |
| PR-16 | Add metrics/logging/dashboard hooks | worker/service/observability | redaction + actionable metrics | integration/manual | disable dashboard rules |
| PR-17 | Add DB override + CLI/API | template tables, CLI, optional handlers | safe fallback to embed | integration/manual | disable override flag |

## 12. Deployment Plan

### Staging deployment steps

1. deploy DB migrations
2. deploy code with all new flags disabled
3. run smoke tests in `legacy` mode
4. enable `EMAIL_TEMPLATE_SOURCE=embed`
5. verify golden-equivalent outputs
6. enable `EMAIL_SHADOW_MODE=true`
7. inspect `email_notifications`
8. enable `EMAIL_NOTIFICATION_ENABLED=true` only in staging
9. cut over auth flows one by one
10. cut over reminder last

### Smoke test checklist

- `SMTP_HOST=""` log-only mode works
- real SMTP staging works
- gửi từng loại email
- check `email_notifications`
- check `email_delivery_attempts`
- check outbox retry
- check logs không có token/OTP/raw link

### Production deployment steps

1. apply DB migrations
2. deploy code with flags safe
3. enable shadow mode for selected auth flows
4. monitor 1 business day
5. enable OTP cutover
6. monitor
7. enable reset cutover
8. monitor
9. enable invitation cutover
10. monitor
11. enable reminder cutover last
12. enable HTML only after text path stable

### Monitoring window

- minimum 24h after each auth cutover
- minimum 48h after reminder cutover

### Rollback trigger

- failure rate spike
- queue lag spike
- duplicate email incidents
- render errors by template
- support reports of missing auth email

### Rollback commands/flags

- set `EMAIL_AUTH_EMAILS_USE_NOTIFICATION_SERVICE=false`
- set `EMAIL_REMINDER_USE_NOTIFICATION_SERVICE=false`
- set `EMAIL_DELIVERY_PATH=legacy`
- set `EMAIL_TEMPLATE_SOURCE=legacy`
- set `EMAIL_FORMAT=text`
- set `EMAIL_NOTIFICATION_ENABLED=false`

### Post-deploy validation

- create one test of each email type
- confirm one notification row
- confirm attempts count expected
- confirm inbox received exactly once

## 13. Definition of Done

- code complete
- unit tests pass
- integration tests pass
- regression tests pass
- no hardcoded template body in business service for migrated flows
- no direct SMTP outside `DeliveryAdapter` for migrated flows
- sensitive vars redacted
- feature flags documented
- rollback tested
- dashboard/alerts ready
- runbook ready
- Tech Lead review complete
- SA review complete
- QA sign-off complete

## 14. Final Recommendation

### Thứ tự ưu tiên bắt buộc

1. **Phase 0**
   Không có baseline test thì không được đụng flow auth/reminder.
2. **Phase 1**
   Tách template nhưng chưa đổi delivery.
3. **Phase 2 + 3**
   Dựng service + repo + worker + adapter + observability tối thiểu.
4. **Phase 4**
   Migrate auth email từng loại.
5. **Phase 5**
   Migrate reminder cuối cùng.
6. **Phase 6**
   HTML + i18n sau khi text path ổn định.
7. **Phase 7**
   DB override/admin tooling có thể defer.
8. **Phase 8**
   Hoàn thiện observability/runbook; phần log/metric cơ bản nên làm sớm hơn, dashboard/alert full có thể finalize sau cutover auth.

### Phase có thể defer

- full DB-driven template
- admin UI
- publish/rollback UI
- đa provider SMTP

### Minimum viable implementation để production an toàn

- phase 0 -> 5
- text templates
- embed registry
- notification tracking tables
- delivery attempts
- worker `email.dispatch`
- auth migration từng loại
- reminder migration sau cùng
- metrics/log redaction cơ bản

### Những điều tuyệt đối không được làm

- không migrate auth và reminder cùng lúc nếu chưa có test
- không xóa legacy path trước khi rollout ổn định
- không persist raw OTP/token/reset link
- không gọi SMTP trực tiếp từ business modules mới
- không publish DB template nếu chưa dry-run render
- không gộp phase 4 và phase 5 vào cùng một PR
- không để reminder retry và email delivery retry chạy song song cho cùng một email

## Appendix — Immediate Jira Seed Suggestion

- Epic A: Baseline and safety rails
- Epic B: Template extraction and rendering
- Epic C: Notification persistence and outbox delivery
- Epic D: Auth email cutover
- Epic E: Reminder cutover
- Epic F: HTML/i18n
- Epic G: Template runtime management
- Epic H: Observability and operational readiness
