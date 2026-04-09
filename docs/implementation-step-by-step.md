# Cobo IAM Services - Step-by-Step Implementation Guide

## 1) Muc tieu tai lieu

Tai lieu nay mo ta trinh tu implement code theo huong incremental, it pha vo, de review, de rollback cho `cobo_iam_services`.

Phuong cham:
- correctness truoc
- tenant isolation truoc
- authorization consistency truoc
- migration safety truoc
- toi uu hoa de sau

## 2) Definition of Done (DoD) chung

Moi buoc duoc coi la hoan thanh khi dap ung du:
- code compile duoc (`go build ./...`)
- test lien quan pass (unit/integration toi thieu cho phan vua them)
- migration chay duoc tren MySQL local
- co log va error wrap co ngu canh
- co kiem tra tenant boundary neu endpoint/business co tenant data
- co audit cho action nhay cam

## 3) P0 - Foundation va Core Access

### Step P0.1 - Bootstrap project va runtime skeleton
**Muc tieu**
- Tao khung project va 2 process: `api` va `worker`.

**Cong viec**
- Tao `go.mod`.
- Tao `cmd/api/main.go`, `cmd/worker/main.go`.
- Tao package platform toi thieu:
  - `internal/platform/config`
  - `internal/platform/logger`
  - `internal/platform/db`
  - `internal/platform/errors`
  - `internal/platform/httpx`
  - `internal/platform/clock`
  - `internal/platform/idgen`

**Acceptance checks**
- `go build ./...` pass.
- API process start/stop duoc (graceful shutdown skeleton).
- Worker process start loop skeleton duoc.

**Rollback**
- Revert nhom file bootstrap, khong anh huong data.

---

### Step P0.2 - MySQL migration batch 0001 (core tables)
**Muc tieu**
- Co schema nen cho IAM, CompanyAccess, Authorization core, Audit, Outbox.

**Cong viec**
- Tao migration `0001_init_core.up.sql` va `0001_init_core.down.sql`.
- Bang toi thieu:
  - IAM: `users`, `credentials`, `sessions`, `login_attempts`
  - CompanyAccess: `companies`, `memberships`, `roles`, `membership_roles`, `departments`, `department_memberships`, `titles`, `membership_titles`
  - Authorization: `permissions`, `role_permissions`, `assignments`, `resource_scope_rules`
  - Audit/Platform: `audit_logs`, `outbox_events`, `idempotency_keys`
- Them index query nong theo plan.

**Acceptance checks**
- Migration up/down chay duoc tren local MySQL 8.
- Unique `memberships(user_id, company_id)` duoc enforce.
- Index can thiet da ton tai.

**Rollback**
- Dung migration down neu chua co data quan trong.
- Neu da co data: dung strategy additive va deprecate, tranh drop gap.

---

### Step P0.3 - Domain contracts va repository interfaces
**Muc tieu**
- Chot interface truoc implementation, giu dependency direction dung.

**Cong viec**
- Tao `domain` va `app` contracts cho:
  - IAM service (`Login`, `Refresh`, `Logout`, `SelectCompany`, `SwitchCompany`)
  - Membership query service
  - Authorizer service (`Authorize`, `GetEffectiveAccess`)
  - Audit append service
  - Outbox publisher
- Chuan hoa model request/response va error codes.

**Acceptance checks**
- Build pass voi interface + stub impl.
- Khong co business logic trong transport.

**Rollback**
- Revert interface layer (khong dong cham DB runtime).

---

### Step P0.4 - IAM login + company selection/switch
**Muc tieu**
- Hoan thanh flow identity + context theo company.

**Cong viec**
- Implement login:
  - verify credential
  - load active memberships
  - 1 company => auto bind
  - nhieu company => tra danh sach de select
  - 0 company => `NO_ACTIVE_COMPANY_ACCESS`
- Implement refresh token rotation.
- Implement select-company/switch-company:
  - verify membership active cua company target
  - issue access token moi co `membership_id + company_id`
  - ghi audit event

**Acceptance checks**
- API:
  - `POST /api/v1/auth/login`
  - `POST /api/v1/auth/refresh`
  - `POST /api/v1/auth/logout`
  - `POST /api/v1/auth/select-company`
  - `POST /api/v1/auth/switch-company`
- Token khong chua full roles/departments/titles.

**Rollback**
- Feature flag endpoint moi; giu schema session on dinh.

---

### Step P0.5 - Authorization core va internal authorize APIs
**Muc tieu**
- Tat ca module dung chung 1 contract authz.

**Cong viec**
- Implement effective access resolver:
  - Effective Permission = union role permissions (+ exceptional grant neu co)
  - Effective Data Scope = union department/assignment/scope rules
  - Effective Responsibility = role/title/department/workflow rule/assignment
- Implement:
  - `POST /internal/v1/authorize`
  - `POST /internal/v1/authorize/batch`
- Bat buoc check tenant boundary truoc.

**Acceptance checks**
- Deny dung ma loi khi company mismatch.
- Cho phep dung khi du permission + scope + responsibility (neu yeu cau action can).

**Rollback**
- Keep old gate (neu co) song song trong thoi gian chuyen tiep.

---

### Step P0.6 - Effective access APIs + me APIs
**Muc tieu**
- Cung cap API cho UI boot context va capability summary.

**Cong viec**
- Implement:
  - `GET /api/v1/me`
  - `GET /api/v1/me/companies`
  - `GET /api/v1/me/effective-access`
  - `GET /api/v1/me/membership`
  - `GET /api/v1/me/capabilities` (optional map tu effective-access)

**Acceptance checks**
- UI co the bootstrap theo current context.
- Backend van authorize lai tai business actions (khong tin capability summary).

**Rollback**
- Co the disable `capabilities` endpoint rieng, giu `effective-access`.

---

### Step P0.7 - Audit module va outbox worker skeleton
**Muc tieu**
- Dam bao traceability va async foundation.

**Cong viec**
- Implement `AppendAuditLog`.
- Hook audit vao cac action:
  - login success/failure, logout
  - select-company/switch-company
  - membership role/department/title changes
- Implement outbox infrastructure:
  - ghi outbox cung transaction business
  - worker poll batch
  - retry + backoff + idempotent consume

**Acceptance checks**
- Co ban ghi audit cho action nhay cam.
- Worker consume outbox event skeleton duoc.

**Rollback**
- Worker co the stop rieng; ghi outbox van ton tai de xu ly lai.

## 4) P1 - Business Modules

### Step P1.1 - Disclosure module
- APIs skeleton: `CreateRecord`, `UpdateRecord`, `SubmitRecord`, `ConfirmRecord`, `ListRecords`, `GetRecord`.
- Moi action nhay cam goi authorizer.
- Moi query tenant-scoped enforce `company_id`.

### Step P1.2 - Workflow module
- Build definition + instance + task lifecycle co optimistic strategy.
- APIs: `CreateWorkflowInstance`, `ApproveTask`, `ReviewTask`, `ConfirmTask`, `ResolveAssignees`.
- Hook assignment/workflow rules vao responsibility.

### Step P1.3 - Notification module
- `ResolveRecipients`, `EnqueueNotification`, `DispatchPending`.
- Recipient co the den tu role/department/title/workflow/direct assignment.
- Dispatch qua outbox-driven worker.

### Step P1.4 - Access administration APIs
- Membership CRUD + role/department/title assignment.
- Role-permission binding + scope/workflow/notification rule admin APIs.
- Audit day du moi thay doi access model.

## 5) P2 - Optimization va Future Hooks

### Step P2.1 - Effective access projection
- Them projection tables/snapshots neu can cho performance.
- Recompute tu outbox events.

### Step P2.2 - Caching
- Redis cache cho effective access: `REDIS_ADDR` + optional password/DB; TTL `EFFECTIVE_ACCESS_CACHE_TTL`; fallback in-memory neu Redis down luc startup.
- Invalidation: hien chi TTL; explicit invalidation khi admin/access model write duoc noi vao projection.

### Step P2.3 - SSO/MFA extension points
- Hook interfaces: `SSOLoginBridge` (TryExternalPrimaryAuth truoc password), `MFACheck` (VerifyAfterPrimaryAuth sau primary auth + account active, truoc list membership / tao session).
- Wire qua `iamapp.NewService(..., iamapp.WithSSOLoginBridge(b), iamapp.WithMFACheck(m))`; mac dinh nil = hanh vi cu.
- `LoginRequest`: `mfa_otp`, `extensions` (JSON); ma loi `MFA_REQUIRED` khi can.

## 6) Test strategy theo giai doan

### Tien do (bootstrap unit tests)
- `internal/iam/app/service_test.go`: login (sai mat khau, 1 company, nhieu company, khong membership active, account khong active), refresh khi chua co company context, MFA hook, SSO bridge.
- `internal/authorization/app/service_test.go`: thieu company context, allow scope department, deny permission, authorize batch.

### P0 tests
- Unit:
  - login rules (1/nhieu/0 company)
  - select/switch company validation
  - authorize decision matrix
- Integration:
  - auth endpoints voi MySQL test db
  - tenant boundary checks
  - migration up/down smoke test

### P1 tests
- Workflow/disclosure permission + state transition tests.
- Notification recipient resolution tests.

### P2 tests
- Projection consistency tests.
- Cache invalidation tests.

## 7) Verification checklist truoc merge moi PR

- `go test ./...` pass
- migration moi co up/down script + indexing notes
- co test cho rule authorization thay doi
- co audit cho endpoint thay doi access model
- docs cap nhat neu thay doi contract

## 8) Suggested implementation order (chi tiet)

1. `P0.1` Bootstrap runtime
2. `P0.2` Migration 0001
3. `P0.3` Contracts/interfaces
4. `P0.4` IAM login/select/switch
5. `P0.5` Internal authorize core
6. `P0.6` Me/effective access APIs
7. `P0.7` Audit + outbox worker skeleton
8. `P1.1` Disclosure
9. `P1.2` Workflow
10. `P1.3` Notification
11. `P1.4` Admin APIs hoan thien
12. `P2.*` Optimization/features mo rong

## 9) Anti-patterns can tranh

- Authorize bang `user_id` khong kem `membership_id + company_id`
- Nhoi roles/permissions day du vao token
- Business logic dat trong HTTP handler
- Query xuyen module khong qua app contracts
- Bo qua audit cho access-sensitive actions

## 10) Requirement Traceability Matrix

Bang nay map yeu cau prompt -> tai lieu -> implementation step de tranh bo sot.

| Prompt requirement group | Covered in this doc | Implementation steps |
|---|---|---|
| Modular monolith + 2 process | Sec 3, Sec 11 | P0.1 |
| Clean/Hexagonal boundaries | Sec 3, Sec 11 | P0.1, P0.3 |
| Multi-company auth principal (`membership_id + company_id`) | Sec 3, Sec 8, Sec 12 | P0.4, P0.5, P0.6 |
| IAM token/session strategy | Sec 3, Sec 12 | P0.4 |
| Effective Permission/Data Scope/Responsibility | Sec 3, Sec 13 | P0.5, P0.6 |
| External auth/authz APIs | Sec 3, Sec 12 | P0.4, P0.6 |
| Internal authorize APIs | Sec 3, Sec 12, Sec 13 | P0.5 |
| Admin access APIs + audit | Sec 4, Sec 12 | P1.4 |
| MySQL schema + indexes | Sec 3, Sec 14 | P0.2, P1.1, P1.2, P1.3, P2.1 |
| Outbox + async worker | Sec 3, Sec 15 | P0.7, P1.3, P2.1 |
| Audit requirements | Sec 3, Sec 16 | P0.7, P1.4 |
| Reliability/production patterns | Sec 2, Sec 17 | P0-P2 (all) |
| Test and verification strategy | Sec 6, Sec 7, Sec 18 | P0-P2 (all) |
| README/TODO/local run deliverables | Sec 19 | P0.1 + P0.2 + P0.7 |

## 11) Module Package Blueprint (final target tree)

Target structure (bat buoc) can duoc tao theo thu tu P0 -> P1:

```text
/cmd
  /api
  /worker

/internal
  /iam
    /domain
    /app
    /infra
    /transport
  /companyaccess
    /domain
    /app
    /infra
    /transport
  /authorization
    /domain
    /app
    /infra
    /transport
  /workflow
    /domain
    /app
    /infra
    /transport
  /disclosure
    /domain
    /app
    /infra
    /transport
  /notification
    /domain
    /app
    /infra
    /transport
  /audit
    /domain
    /app
    /infra
    /transport
  /platform
    /db
    /config
    /logger
    /authctx
    /events
    /outbox
    /clock
    /idgen
    /errors
    /httpx
    /mysqlx
```

Dependency rules:
- `domain` khong import `infra` va `transport`
- `app` chi dung interfaces/repository contracts
- `infra` implement contracts (mysql, jwt, password hash, outbox sink...)
- `transport` parse/validate request, khong chua business logic

## 12) API Contract Matrix (ready-for-implementation)

Chi tiet request/response JSON day du (mau body): `docs/api-contracts-json.md`.

### 12.1 External APIs (UI/client)

| API | Purpose | Auth required | Success | Error codes (chinh) | Phase |
|---|---|---|---|---|---|
| `POST /api/v1/auth/login` | Authenticate + return company context or selectable memberships | No | 200 | `INVALID_CREDENTIALS`, `ACCOUNT_LOCKED`, `NO_ACTIVE_COMPANY_ACCESS` | P0 |
| `POST /api/v1/auth/refresh` | Rotate access token from refresh token | No | 200 | `SESSION_EXPIRED` | P0 |
| `POST /api/v1/auth/logout` | Revoke current refresh/session | Yes/Token-based | 200 | `SESSION_EXPIRED` | P0 |
| `GET /api/v1/me` | Return identity + current context | Yes | 200 | `COMPANY_CONTEXT_REQUIRED` | P0 |
| `GET /api/v1/me/companies` | List active memberships/companies | Yes | 200 | `PERMISSION_DENIED` | P0 |
| `POST /api/v1/auth/select-company` | Bind first company context after login | Yes/pre-company | 200 | `MEMBERSHIP_NOT_FOUND`, `COMPANY_SCOPE_MISMATCH` | P0 |
| `POST /api/v1/auth/switch-company` | Rotate token to new company context | Yes | 200 | `MEMBERSHIP_NOT_FOUND`, `COMPANY_SCOPE_MISMATCH` | P0 |
| `GET /api/v1/me/effective-access` | Return effective permission/scope/responsibility | Yes | 200 | `COMPANY_CONTEXT_REQUIRED` | P0 |
| `GET /api/v1/me/capabilities` | Optional menu capability summary | Yes | 200 | `COMPANY_CONTEXT_REQUIRED` | P0 |
| `GET /api/v1/me/membership` | Return roles/departments/titles in current company | Yes | 200 | `COMPANY_CONTEXT_REQUIRED` | P0 |

Status code baseline:
- 200, 201, 400, 401, 403, 404, 409, 422

### 12.2 Internal authorization APIs

| API | Purpose | Input | Output | Phase |
|---|---|---|---|---|
| `POST /internal/v1/authorize` | Single authorization decision | subject + action + resource | allow/deny + reasons | P0 |
| `POST /internal/v1/authorize/batch` | Batch decisions | subject + checks[] | per-check decisions | P0 |

### 12.3 Admin APIs (minimum)

| Group | APIs | Phase |
|---|---|---|
| Membership mgmt | create/update/delete/list memberships | P1 |
| Role assignment | add/remove membership roles | P1 |
| Department assignment | add/remove membership departments | P1 |
| Title assignment | add/remove membership titles | P1 |
| Permission catalog | list permissions, list roles, bind/unbind role permissions | P1 |
| Scope/rule setup | create resource-scope/workflow-assignee/notification rules | P1 |

All admin APIs MUST append audit logs.

## 13) Internal Authorization Models (canonical)

`AuthorizeRequest`
- `subject.user_id`
- `subject.membership_id`
- `subject.company_id`
- `action`
- `resource.type`
- `resource.id`
- `resource.attributes` (optional map)
- `context` (optional)

`AuthorizeDecision`
- `decision` (`allow`/`deny`)
- `matched_permissions[]`
- `scope_reasons[]`
- `responsibility_reasons[]`
- `deny_reason_code` (nullable)

Mandatory guard order for every authorize path:
1. authenticated/session valid
2. user active
3. membership active
4. company boundary match
5. permission match
6. data scope match
7. responsibility/state constraints

## 14) Schema and Index Matrix (MySQL 8)

### 14.1 Table batches by migration

- `0001_init_core`: IAM + CompanyAccess + Authorization core + Audit + Outbox + Idempotency
- `0002_business_modules`: Workflow + Disclosure + Notification
- `0003_projection_opt`: effective access projections/snapshots + optimization indexes

### 14.2 Entity -> table mapping

| Entity | Table |
|---|---|
| User | `users` |
| Credential | `credentials` |
| Session | `sessions` |
| Company | `companies` |
| Membership | `memberships` |
| Role | `roles` |
| MembershipRole | `membership_roles` |
| Department | `departments` |
| DepartmentMembership | `department_memberships` |
| Title | `titles` |
| MembershipTitle | `membership_titles` |
| Permission | `permissions` |
| RolePermission | `role_permissions` |
| Assignment | `assignments` |
| ResourceScopeRule | `resource_scope_rules` |
| WorkflowDefinition | `workflow_definitions` |
| WorkflowStepDefinition | `workflow_step_definitions` |
| WorkflowInstance | `workflow_instances` |
| WorkflowStepInstance | `workflow_step_instances` |
| WorkflowAssigneeRule | `workflow_assignee_rules` |
| Task | `tasks` |
| DisclosureRecord | `disclosure_records` |
| DisclosureHistory | `disclosure_histories` |
| NotificationRule | `notification_rules` |
| NotificationJob | `notification_jobs` |
| NotificationDelivery | `notification_deliveries` |
| AuditLog | `audit_logs` |
| OutboxEvent | `outbox_events` |
| IdempotencyKey | `idempotency_keys` |

### 14.3 Required index matrix

| Table | Required indexes |
|---|---|
| `memberships` | unique(`user_id`,`company_id`), idx(`company_id`,`membership_status`), idx(`user_id`,`membership_status`) |
| `membership_roles` | idx(`membership_id`,`status`,`effective_from`,`effective_to`) |
| `department_memberships` | idx(`membership_id`,`status`,`effective_from`,`effective_to`), idx(`department_id`,`status`) |
| `assignments` | idx(`company_id`,`assignee_type`,`assignee_ref_id`,`status`), idx(`company_id`,`resource_type`,`resource_id`,`status`) |
| `disclosure_records` | idx(`company_id`,`status`,`created_at`), idx(`company_id`,`department_id`,`status`,`created_at`) |
| `tasks` | idx(`company_id`,`assignee_membership_id`,`status`), idx(`company_id`,`workflow_instance_id`,`step_code`) |
| `audit_logs` | idx(`company_id`,`occurred_at`), idx(`actor_user_id`,`occurred_at`), idx(`resource_type`,`resource_id`,`occurred_at`), idx(`request_id`) |

## 15) Outbox and Async Processing Baseline

Outbox table fields (minimum):
- `event_id`
- `aggregate_type`
- `aggregate_id`
- `event_type`
- `payload_json`
- `status`
- `available_at`
- `created_at`
- `processed_at`

Worker behavior baseline:
- polling by batch (small fixed batch at start)
- ordered by `available_at`, then `created_at`
- retry with exponential backoff + jitter
- idempotent consumer handlers
- dedupe by event key/idempotency key when needed
- dead-letter strategy (status `failed_permanent`) for repeated terminal failures

## 16) Audit Coverage Matrix

Must log events:
- login success/failure
- logout
- company selection
- switch company
- create/update/delete membership
- role/department/title assignment changes
- notification rule changes
- workflow assignee rule changes
- create/update/confirm disclosure
- permission-sensitive actions

Audit payload minimum fields:
- `event_id`, `occurred_at`
- `actor_user_id`, `actor_membership_id`, `company_id`
- `action`, `resource_type`, `resource_id`, `decision`
- `request_id`, `ip`, `user_agent`
- `effective_permissions_snapshot`, `effective_scope_snapshot`, `metadata_json`

## 17) Reliability and Observability Baseline

Mandatory defaults:
- timeout for all I/O paths via `context.Context`
- retry only idempotent operations
- exponential backoff + jitter for async retry
- structured logging everywhere
- request_id propagation API -> app -> infra
- trace_id propagation via OpenTelemetry hooks
- graceful shutdown for `api` and `worker`
- clear transaction boundary per use case
- no distributed transaction
- idempotency key for sensitive confirm/approve/submit actions

Risk priority while implementing:
1. crash risk
2. data corruption
3. tenant isolation
4. security
5. reliability
6. performance
7. maintainability

## 18) Verification and Test Expansion

### 18.1 Authorization regression suite (must-have)
- company mismatch -> deny
- inactive membership -> deny
- permission allow but scope deny -> deny
- permission + scope + responsibility all match -> allow

### 18.2 Migration safety suite
- clean database migrate up to latest
- rollback one step then forward again
- index existence check
- representative query explain check on hot paths

### 18.3 Worker reliability suite
- outbox retry behavior
- idempotent re-processing same event
- poison message to failed-permanent flow

## 19) Deliverables and Ownership Checklist

Deliverables expected by end of P0:
- project tree skeleton
- `go.mod`
- sample config file
- migration SQL (0001)
- domain entities core
- repository interfaces
- mysql repository skeleton
- http router/handler skeleton
- auth middleware
- authorization module skeleton
- IAM login/select/switch flows
- audit append baseline
- outbox worker skeleton
- `README.md` architecture + local run
- `TODO.md` next tasks

P1 adds:
- disclosure/workflow/notification skeleton + core flows
- admin APIs complete for access model operations

P2 adds:
- projection optimization, caching, SSO/MFA hooks

## 20) Local Run and README Plan (must include in implementation)

README sections to include:
1. architecture overview
2. module boundaries
3. how to run MySQL locally
4. migration commands
5. run API process
6. run Worker process
7. test commands
8. env/config keys
9. known limitations

## 21) Final Implementation Order (authoritative)

1. P0.1 bootstrap
2. P0.2 migration 0001
3. P0.3 contracts/interfaces
4. P0.4 IAM login/refresh/logout/select/switch
5. P0.5 internal authorize and resolver
6. P0.6 me/effective-access/capabilities/membership endpoints
7. P0.7 audit + outbox worker skeleton
8. P1 disclosure
9. P1 workflow
10. P1 notification
11. P1 admin APIs
12. P2 projection/cache/SSO-MFA hooks

