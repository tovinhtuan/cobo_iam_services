# P0-1 Implementation Plan — Align Proposal Deadline Contract Cross-Repo

## Objective

Fix mismatch giữa:

- FE meaning: `proposed_deadline_days`
- API wire name: `proposed_deadline_date`
- DB persistence: `proposed_deadline_date DATE`

Mục tiêu của plan này là đủ chi tiết để bắt đầu implementation ngay sau khi chốt semantics business.

---

## Decision To Lock First

Trước khi code, phải chốt 1 trong 2 hướng:

### Option A — Proposal stores deadline as number of days

Ý nghĩa:

- user nhập số ngày
- API dùng field `proposed_deadline_days`
- DB lưu số ngày

### Option B — Proposal stores deadline as absolute date

Ý nghĩa:

- user nhập ngày cụ thể
- API dùng field `proposed_deadline_date`
- DB lưu ngày

## Recommendation

Với current UI và current wording hiện có, **Option A** hợp lý hơn vì:

- FE hiện đang thu number input
- field name ở FE đã theo `days`
- business review trước đó cũng đang nói theo duration hơn là exact date

Nếu product không phản đối, nên đi theo:

**Option A: normalize toàn hệ thống sang `proposed_deadline_days`**

---

## Contract-First Proposal

## Target contract

### Create proposal request

```json
{
  "type_id": "dt-001",
  "title": "CBTT bất thường",
  "description": "Mô tả ngắn",
  "proposed_t0_date": "2026-06-01",
  "proposed_deadline_days": 5,
  "step_overrides": [
    { "step_id": "step-review", "processing_days": 2 }
  ],
  "process_controller_membership_id": "m_123"
}
```

### Proposal response

```json
{
  "proposal_id": "prop-001",
  "type_id": "dt-001",
  "status": "ad_hoc_draft",
  "title": "CBTT bất thường",
  "description": "Mô tả ngắn",
  "proposed_t0_date": "2026-06-01",
  "proposed_deadline_days": 5,
  "process_controller_id": "m_123"
}
```

### Backward compatibility

Ngắn hạn có thể:

- accept cả `proposed_deadline_date` cũ ở request như compatibility shim
- nhưng normalize internal sang `proposed_deadline_days`
- response mới chỉ nên trả field canonical

---

## Affected Areas

## Backend

- `internal/adhoc/app/contracts.go`
- `internal/adhoc/app/service.go`
- `internal/adhoc/infra/mysql/repository.go`
- `internal/adhoc/transport/http/handler.go`
- migrations for `ad_hoc_proposals`
- tests:
  - `internal/adhoc/app/service_test.go`
  - `internal/adhoc/transport/http/handler_test.go`
  - any integration tests touching proposal contract

## Frontend

- `../cobo_web_design/src/services/adHocAlertsApi.ts`
- `../cobo_web_design/src/services/workflowOverrideMappers.ts`
- `../cobo_web_design/src/types.ts`
- `../cobo_web_design/src/pages/portal/AdHocProposalCreatePage.tsx`
- `../cobo_web_design/src/pages/portal/AdHocProposalDetailPage.tsx`
- tests:
  - `../cobo_web_design/src/services/workflowServices.contract.test.ts`
  - `../cobo_web_design/src/pages/portal/portal.workflow.regression.test.tsx`

## Docs

- `docs/api-contracts-json.md`
- `../cobo_web_design/docs/canh-bao-bat-thuong-feature-doc.md`
- `../cobo_web_design/docs/permission_catalog.md` only if wording references deadline semantics

---

## Implementation Phases

## Phase 0 — Confirm semantics

### Output

- one-line decision:
  - `proposal deadline uses number-of-days semantics`

### Acceptance

- BA/product/dev all agree to canonical meaning

---

## Phase 1 — Backend contract and persistence update

### Tasks

1. Update create/list/detail DTOs in `internal/adhoc/app/contracts.go`
2. Replace `ProposedDeadline` / `ProposedDeadlineDate` with canonical `ProposedDeadlineDays` naming
3. Update repository read/write mapping
4. Decide DB shape:
   - preferred: add `proposed_deadline_days INT NULL`
   - optional migration path: keep old date column temporarily for compatibility migration

### Recommendation

Do not overload `DATE` column with day count.

Preferred migration:

- add `proposed_deadline_days INT NULL`
- backfill if needed
- stop writing old `proposed_deadline_date`
- later remove deprecated column in separate migration

### Acceptance

- backend persists day-count in type-safe, self-documenting form
- response and request contracts expose canonical field

---

## Phase 2 — FE contract cleanup

### Tasks

1. Update `CreateAdHocProposalInput`
2. Update `toCreateProposalWireBody`
3. Update proposal DTO normalization
4. Update create page labels/messages only if needed for clarity
5. Update detail page display to read canonical field

### Acceptance

- FE request body uses canonical deadline field
- FE no longer depends on deprecated wire field name

---

## Phase 3 — Compatibility handling

### Tasks

1. Decide if BE should accept old field for one release cycle
2. If yes, add request parsing compatibility shim
3. Add explicit TODO / doc for removal window

### Acceptance

- old clients either still work intentionally, or break clearly with documented reason

---

## Phase 4 — Tests and docs

### Tasks

1. Update FE contract tests
2. Update FE regression tests
3. Update BE unit/integration tests
4. Update docs

### Acceptance

- all proposal contract tests reflect canonical semantics
- docs no longer mention conflicting field names

---

## Suggested Migration Strategy

## Safe additive path

1. Add new DB column `proposed_deadline_days`
2. Update backend to write/read canonical field
3. Optionally read fallback from old column for old rows
4. Update FE to send canonical field
5. Backfill old rows if needed
6. Remove deprecated column in later cleanup ticket

This is safer than in-place mutation of `DATE` semantics.

---

## Risks

### Risk 1

Product actually wants absolute date, not day count.

Mitigation:

- do not start code before decision is signed off

### Risk 2

Existing demo/seed data may contain old field assumptions.

Mitigation:

- verify migration/backfill strategy on local DB

### Risk 3

FE tests may still mock old flat payload shape.

Mitigation:

- update contract tests before or together with implementation

---

## Verification Plan

## Backend

- targeted Go tests for adhoc module
- integration test for create/get/list proposal
- `docker compose -f docker-compose.dev.yml build api`

## Frontend

- update and run contract tests
- update and run portal regression tests
- `npm run build` in `../cobo_web_design`

## End-to-end manual

1. open create page
2. create proposal with deadline days
3. open detail
4. verify displayed deadline value matches submitted value
5. verify DB/API readback uses canonical semantics

---

## Task Breakdown

### Task A

Decision note on semantics

Size: `S`

### Task B

Backend additive contract + migration

Size: `M`

### Task C

Frontend contract/mapping update

Size: `S`

### Task D

Tests + docs + verify

Size: `S`

Overall expected size:

`M`

---

## Ready-To-Start Checklist

- [ ] Business semantics confirmed: days vs date
- [ ] Canonical field name approved
- [ ] Additive migration strategy approved
- [ ] FE and BE owners assigned
- [ ] Test commands agreed
