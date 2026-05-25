# Phase 5 — Execution Plan: Màn «Chi tiết cảnh báo» (dev-ready)

**Ngày:** 2026-05-25  
**Ước lượng:** ~8–12 dev-days (1 FE + 0.5 BE), tùy BE notes/timelines  
**Prerequisite:** Phase 2–3 đã deploy (`GET /company/deadline-alerts`, `DeadlineList` API, `DeadlineDetail` load tối thiểu).

**Canonical PO:** `deadline-alert-detail-po-decisions-summary.md` §0  
**Contract alignment:** `deadline-alert-detail-contract-alignment-assessment.md`

---

## 0. Mục tiêu & không làm

### 0.1 Mục tiêu

Thay toàn bộ **mock** trong `DeadlineDetail.tsx` bằng dữ liệu thật, giữ **cockpit UI** (timeline + workflow cards + sidebar + footer), bám:

- **§0 PO:** Hoàn tất = **workflow task xong** + **công bố hồ sơ (có `evidence_link`)** → alert `DONE`.
- **P3/P4/H3:** `record_id`, `active_departments`, không derive T0 trên FE.
- **Cắt scope PO:** bỏ dịch thuật (10=A); ẩn email/InfoBox bước 4 (11=A).
- **OQ-DA-06:** List header → «Tạo cảnh báo bất thường» → `/app/ad-hoc-proposals/new`.

### 0.2 Không làm

- `POST /api/v1/company/deadline-alerts/.../complete`
- `setStatus('Done')` / `completedStages` local
- Endpoint aggregated alert-detail (optional sau)
- UI đặc thù bước 4 (email/InfoBox)
- Module dịch thuật

---

## 1. Runtime contract (đã verify code — dev bám đúng path)

### 1.1 Load detail

| Thứ tự | API | Permission | Ghi chú |
|--------|-----|------------|---------|
| 1 | `GET /api/v1/disclosures/{record_id}` | `disclosure.view` | Header, attachments, `workflow_instance_id` |
| 2 | `GET /api/v1/company/deadline-alerts?page=1&pageSize=200` | `deadline.view` | Match `alert_id`/`record_id`; ưu tiên `location.state.alert` từ list |
| 3 | `GET /api/v1/workflows/instances/{id}` | `workflow.read` | `snapshot[]`, `current_step_code`, `status`, `t0_date` |
| 4 | `GET /api/v1/workflows/instances/{id}/tasks` | `workflow.read` | Task list cho toggle/actions |
| 5 | `GET /api/v1/disclosure-types/{type_id}` | type read | Sidebar metadata |

**BE response workflow (fact):** `WorkflowInstanceDTO` có `snapshot[]`, `current_step_code` — **không** có `workflow[]`, `step_timelines` trong JSON BE hiện tại. FE mapper đã hỗ trợ cả hai nếu có.

### 1.2 Map `snapshot` → steps UI (bắt buộc)

```typescript
// snapshot[i] (BE StepSnapshot) → WorkflowOverrideStepDto-like
{
  step_id: snap.step_id,
  stage: snap.stage || snap.step_code,
  department_id: undefined, // dùng snap.department (string name) hiển thị pill
  processing_days: snap.processing_days,
  display_order: snap.display_order,
  documents: [], // load từ type workflow nếu cần doc list (optional)
}
```

**Current step:** `instance.current_step_code` khớp `snap.step_code` hoặc `step_id`.

**Progress %:** `completedSteps / totalSteps` — step `< current` = completed; `=== current` = current.

### 1.3 Hoàn tất (§0 PO) — API thật

| Bước | API | Permission | Code tham chiếu |
|------|-----|------------|-----------------|
| 1 | `PATCH /api/v1/disclosures/{record_id}` body có `evidence_link` | `disclosure.update` | `disclosureApi.update` |
| 2 | `POST /api/v1/disclosures/{record_id}/submit` | `disclosure.submit` | BE `SubmitRecord` → `status=Published` | `DisclosureForm.tsx` L313–316 |

**Lưu ý naming:** BE `SubmitRecord` set **`Published`** (không qua `submitted`→`confirmed` trên path này). FE label «Công bố» — giữ parity `DisclosureForm`.

**Sau publish:** `GET /company/deadline-alerts` → item `status=DONE` (record `published`|`completed`).

**Optional sau publish:** `POST /api/v1/disclosures/{id}/confirm` → `Completed` (nếu product cần thêm bước «hoàn tất hồ sơ»).

**Fix bắt buộc (nếu submit fail):** `disclosureApi.submit` hiện gọi `/api/v1/records/{id}/submit` — BE là `/api/v1/disclosures/{id}/submit`. Ticket **FE-DA-D20**.

### 1.4 Workflow task (toggle / nút bước)

| Action | API |
|--------|-----|
| review | `POST /api/v1/workflows/tasks/{task_id}/review` |
| approve | `POST .../approve` |
| confirm | `POST .../confirm` |
| reject | `POST .../reject` |

FE: `workflowInstancesApi.actOnTask(taskId, action, { comment? })`.

**Toggle «Hoàn thành» trên card:** **không** PATCH riêng — UI reflect `task.status === 'confirmed'` (hoặc bước cuối); click mở nút action hợp lệ (copy `DisclosureDetail.getTaskActions`).

### 1.5 Điều kiện enable «Xác nhận kết thúc» / sidebar publish

```text
canPublish =
  user.permissions includes 'disclosure.publish'
  AND user.permissions includes 'disclosure.submit'
  AND record.status NOT IN ('Published','Completed','published','completed')
  AND evidenceLink.trim() !== ''  // hoặc bắt nhập trong modal
  AND allRequiredTasksTerminal(workflowTasks)  // không còn task pending/reviewed/approved chưa confirm
```

`allRequiredTasksTerminal`: mọi task `confirmed` hoặc `rejected` (instance `completed`/`rejected` → disable actions).

### 1.6 Ghi chú sidebar (OQ-09 HC-1)

**Fact:** `RecordDTO` **không** có field `notes`.

| Phase | Cách làm |
|-------|----------|
| **5A (ship nhanh)** | «Cập nhật thông tin» → `navigate(/app/disclosures/{id}/edit)` hoặc modal PATCH `summary` (product chấp nhận ghi vào summary) |
| **5B (đúng contract)** | BE migration + `PATCH` field `internal_notes` — ticket **BE-DA-D12** |

Plan dev: implement **5A** trước; song song BE-DA-D12 nếu PO giữ HC-1 notes persist.

### 1.7 `step_timelines` & `remaining_days` (OQ-06/12 HC-1)

| Field | Hiện trạng | Dev action |
|-------|------------|------------|
| `step_timelines` | BE instance GET **không trả** | FE fallback: label `Due: T+{processing_days}` từ `snapshot.due_rule` hoặc `processing_days`; ticket **BE-DA-D13** enrich optional |
| `remaining_days` | Chưa có trên alert API | Hiển thị từ alert list `dueDate` vs today, hoặc ẩn field «Thời hạn còn lại» đến BE-DA-D13 |

---

## 2. Cấu trúc file mới / sửa

### 2.1 FE (`cobo_web_design`)

| File | Việc |
|------|------|
| `src/services/deadlineAlertDetailViewModels.ts` | **NEW** — VM + step status mapper |
| `src/services/deadlineAlertDetailViewModels.test.ts` | **NEW** |
| `src/services/workflowOverrideMappers.ts` | Thêm `current_step_code` vào `normalizeWorkflowInstanceDto` |
| `src/services/snapshotToWorkflowSteps.ts` | **NEW** — `snapshotToDisplaySteps(snapshot, currentStepCode)` |
| `src/services/disclosureApi.ts` | Fix submit path (**D20**) |
| `src/pages/portal/DeadlineDetail.tsx` | Refactor chính |
| `src/pages/portal/DeadlineList.tsx` | `navigate state` + CTA ad-hoc (**D19**) |
| `src/components/deadline/WorkflowStepsOverview.tsx` | **NEW** — timeline bar + milestones |
| `src/components/deadline/DeadlineWorkflowCard.tsx` | **NEW** — card 1 bước + task actions |
| `src/components/deadline/PublishDisclosureModal.tsx` | **NEW** — evidence input + submit |

### 2.2 BE (`cobo_iam_services`) — optional trong sprint

| File | Việc |
|------|------|
| `internal/workflow/transport/http/handler.go` | Enrich GET instance: `step_timelines`, map `snapshot`→`workflow` (**D13**) |
| `internal/disclosure/...` | `internal_notes` (**D12**) |
| `internal/deadlinealerts/...` | `?record_id=` filter (**D01**) |

---

## 3. Thứ tự triển khai (step-by-step)

### STEP 0 — Chuẩn bị (0.5 ngày)

- [ ] Đọc §0–1 file này + PO summary.
- [ ] Tạo branch `feature/deadline-detail-phase5`.
- [ ] Record test `record_id` trên dev: ad-hoc approved có workflow (xem `adhoc-approved-not-in-deadlines-tab-summary.md`).
- [ ] Verify permissions test user: `deadline.view`, `disclosure.view`, `workflow.read`, `workflow.review|approve|confirm`, `disclosure.publish`, `disclosure.submit`, `disclosure.update`.

**Verify:**

```bash
# FE
cd cobo_web_design && npm run build

# BE (nếu sửa)
cd cobo_iam_services && go test ./internal/deadlinealerts/... ./internal/workflow/...
```

---

### STEP 1 — Quick wins list (0.5 ngày) `FE-DA-D19`, `FE-DA-D20` ✅ (2026-05-25)

#### 1.1 `DeadlineList.tsx` — done

- Label: **«Tạo cảnh báo bất thường»** → `/app/ad-hoc-proposals/new`.
- Permission: `ad_hoc_alert.propose` (tab Deadlines only).
- Tab History: giữ «Tạo cảnh báo mới» + `disclosure.create` + `/app/disclosures/new`.

#### 1.2 `disclosureApi.ts` — done

- `submit` → `POST /api/v1/disclosures/{recordId}/submit`.
- Test: `disclosureApi.contract.test.ts` updated, 2 passed.

**AC:** `DisclosureForm` publish dùng path BE đúng.

---

### STEP 2 — Foundation load (1 ngày) `FE-DA-D01`–`D04`, `D02` ✅ (2026-05-25)

#### 2.1 `normalizeWorkflowInstanceDto`

Thêm field:

```typescript
current_step_code: asString(root.current_step_code),
```

Map `workflow` từ `snapshot` khi `root.workflow` rỗng:

```typescript
workflow: mapSnapshotToWorkflowSteps(root.snapshot ?? root.Snapshot),
```

#### 2.2 `deadlineAlertDetailViewModels.ts`

```typescript
export type WorkflowStepUiStatus = 'completed' | 'current' | 'upcoming';

export function resolveStepUiStatus(
  stepCode: string,
  currentStepCode: string,
  stepOrder: number,
  currentOrder: number,
): WorkflowStepUiStatus { ... }

export function toDeadlineAlertDetailVM(input: {
  alert: DeadlineAlert | null;
  record: DisclosureRecord;
  instance: WorkflowInstanceDto | null;
  tasks: WorkflowTaskDto[];
  typeName?: string;
}): DeadlineAlertDetailVM { ... }
```

#### 2.3 `DeadlineDetail.tsx` — load pipeline

```typescript
const [workflowInstance, setWorkflowInstance] = useState<WorkflowInstanceDto | null>(null);
const [workflowTasks, setWorkflowTasks] = useState<WorkflowTaskDto[]>([]);

// loadDetail:
// 1) disclosure getById
// 2) alert from location.state ?? list match
// 3) if workflowInstanceId: getById + list tasks
// 4) disclosureTypes get type name
```

- **Xóa** state: `completedStages`, `setStatus` local cho Done.
- `status` badge chỉ từ `alert.status`.

#### 2.4 `DeadlineList.tsx` navigation

```typescript
navigate(`/app/deadlines/${alert.id}`, { state: { alert } });
```

**AC:**

- Deep link vẫn load (list fallback).
- Unit tests view model 3 cases: no workflow, mid-flight, Done.

#### 2.5 Shipped (2026-05-25)

- `snapshotToWorkflowSteps.ts` + `normalizeWorkflowInstanceDto` map `snapshot` → `workflow`, `current_step_code` trên `WorkflowInstanceDto`.
- `deadlineAlertDetailViewModels.ts` + tests (11 passed với `snapshotToWorkflowSteps.test.ts`).
- `DeadlineDetail.tsx`: load disclosure + alert (`location.state` / list) + instance + tasks + type name; badge/progress/timeline từ VM; bỏ `setStatus('Done')` / toggle local (STEP 4 wire publish).
- `DeadlineList.tsx`: `Link state={{ alert: source }}`.

---

### STEP 3 — Timeline & cards read-only (2 ngày) `FE-DA-D05`–`D08`, `D06`, `D18` ✅ (2026-05-25)

#### 3.1 Component `WorkflowStepsOverview`

- Input: `steps: { title, dateLabel, status }[]`, `progressPercent`.
- Thay block `milestones[]` hardcoded + animation % cố định.
- `dateLabel`: từ `step_timelines` nếu có; else `formatDueRule(snap.due_rule)` hoặc `—`.

#### 3.2 Component `DeadlineWorkflowCard`

- Props: `step`, `tasksForStep`, `departmentLabel`, `documents`, `isReadOnly`.
- **Toggle:** read-only switch `checked={stepStatus==='completed'}` — **không** `onToggle` local.
- Task action buttons (STEP 4 có thể enable).
- **Bỏ** DocumentList mock; map `step.documents` + `record.attachments` (bước publish nếu có).

#### 3.3 Department label `FE-DA-D06`

- Ưu tiên `alert.activeDepartments[0]`.
- Fallback: `snapshot.department` string.
- Optional: `disclosureTypesApi.listCompanyGroups({ departmentId })` nếu cần id→name.

#### 3.4 Sidebar metadata `FE-DA-D07`

- Loại CBTT, planned date, record status, `template_category` từ alert.
- **Xóa** widget Dịch thuật (**D18**).
- Bước 4 card: **không** render email/InfoBox (**D18**).

#### 3.5 Cleanup `FE-DA-D08`

- Xóa `handleFinish`, `handleUpdate` toast giả, Settings/ExternalLink no-op (hoặc wire STEP 5).
- Giữ banner «Xem hồ sơ đầy đủ».

**AC:** Manual — ad-hoc record hiển thị N bước = `snapshot.length`, không phải luôn 4.

#### 3.6 Shipped (2026-05-25)

- `WorkflowStepsOverview.tsx` — timeline + progress từ VM (N bước).
- `DeadlineWorkflowCard.tsx` — read-only toggle, tasks list, documents từ snapshot + attachments (publish step).
- `DeadlineAlertDetailSidebar.tsx` — metadata; bỏ widget Dịch thuật; publish card không email/InfoBox.
- `DeadlineDetail.tsx` — refactor; footer «Cập nhật hồ sơ» → edit; header Settings/ExternalLink wired; «Xác nhận kết thúc» disabled đến STEP 5.

---

### STEP 4 — Workflow actions (1.5 ngày) `FE-DA-D15` (phần task) ✅ (2026-05-25)

Copy pattern từ `DisclosureDetail.tsx`:

- `getTaskActions(task)` theo permission `workflow.review` / `approve` / `confirm`.
- `handleTaskAction` → `actOnTask` → reload instance + tasks.
- Hiển thị lỗi `formatWorkflowApiErrorMessage`.

**AC:** User có quyền confirm task → trạng thái card/toggle cập nhật sau reload.

#### 4.1 Shipped (2026-05-25)

- `workflowTaskActions.ts` — `getWorkflowTaskActions`, labels VI, permission helper + unit tests.
- `DeadlineDetail.tsx` — `handleTaskAction`, `reloadWorkflow`, banners success/error.
- `DeadlineWorkflowCard.tsx` — nút Rà soát / Phê duyệt / Xác nhận / Từ chối trên từng task.

---

### STEP 5 — Publish & hoàn tất (1.5 ngày) `FE-DA-D15` (phần publish) ✅ (2026-05-25)

#### 5.1 `PublishDisclosureModal.tsx`

- Input bắt buộc: `evidenceLink` (URL SSC/HNX).
- Optional: hiển thị checklist «Đã có bằng chứng công bố».
- On confirm:
  1. `disclosureApi.update(recordId, { ...record, evidenceLink })`
  2. `disclosureApi.submit(recordId, crypto.randomUUID())`
  3. Reload alert + record → expect Done

#### 5.2 Wire UI

| Control | Handler |
|---------|---------|
| Footer «Xác nhận kết thúc» | Mở modal nếu `canPublish` |
| Sidebar «Đã Công bố/Báo cáo đúng hạn» | Cùng modal |
| Disabled + tooltip | Khi thiếu evidence / task chưa xong / thiếu quyền |

#### 5.3 Footer «Cập nhật thông tin»

- **5A:** `navigate(/app/disclosures/${id}/edit)` hoặc modal notes→`summary` PATCH.
- **5B:** `FE-DA-D16` khi BE-DA-D12 xong.

#### 5.4 Header `FE-DA-D08` / Settings

- **ExternalLink:** `window.open(/app/history/${id})`.
- **Settings:** `navigate(/app/disclosures/${id}/edit#reminders)` hoặc hide nếu chưa có anchor.

**AC E2E:**

1. Approve all tasks → modal publish → nhập evidence → submit.
2. Alert badge **Đã hoàn thành**; sidebar filled green; không reload mock state.
3. User không có `disclosure.publish` → nút disabled.

#### 5.5 Shipped (2026-05-25)

- `PublishDisclosureModal.tsx` — evidence URL + checklist; `update` + `submit`.
- `deadlinePublishReadiness.ts` — quyền, task confirmed, trạng thái hồ sơ.
- Footer + sidebar mở modal; reload record/alert/workflow sau công bố.

---

### STEP 6 — Documents per step (0.5 ngày) `FE-DA-D17`

- `DocumentList` read-only từ `step.documents`.
- Nút «+» → `navigate(/app/disclosures/${recordId}/edit)` (upload ở hồ sơ).

---

### STEP 7 — BE optional (song song hoặc sprint+1)

| Ticket | Dev | AC |
|--------|-----|-----|
| **BE-DA-D01** | `GET deadline-alerts?record_id=` | Deep link 1 query |
| **BE-DA-D13** | GET instance + `step_timelines` từ `ComputeStepTimelines` | FE hiện ngày thật |
| **BE-DA-D12** | `internal_notes` on record | FE-DA-D16 sidebar save |

---

### STEP 8 — QA & regression (1 ngày)

Checklist:

- [ ] Tab History không đổi.
- [ ] `DisclosureDetail` workflow không break sau `D02`.
- [ ] Periodic record không workflow → banner degraded (OQ-DA-03).
- [ ] `draft` không trong list.
- [ ] Permission `deadline.view` 403 message rõ.
- [ ] Không còn `setState Done` trong `DeadlineDetail`.

```bash
cd cobo_web_design && npm test -- --run src/services/deadlineAlertDetailViewModels.test.ts src/services/deadlineAlertsApi.test.ts
cd cobo_web_design && npm run build
```

---

## 4. Ticket checklist (copy vào Jira)

| ID | Step | SP gợi ý |
|----|------|----------|
| FE-DA-D19 | 1 | 1 |
| FE-DA-D20 | 1 | 1 |
| FE-DA-D01–D04 | 2 | 3 |
| FE-DA-D02 | 2 | 1 |
| FE-DA-D05–D08 | 3 | 5 |
| FE-DA-D06 | 3 | 2 |
| FE-DA-D18 | 3 | 1 |
| FE-DA-D15 (tasks) | 4 | 3 |
| FE-DA-D15 (publish) | 5 | 3 |
| FE-DA-D17 | 6 | 2 |
| FE-DA-D16 | 5B | 2 |
| BE-DA-D01 | 7 | 2 |
| BE-DA-D12–D13 | 7 | 5 |

---

## 5. Definition of Done (release)

- [ ] `DeadlineDetail` không còn mock milestones/forms/docs.
- [ ] §0 PO: publish có evidence → Done trên alert.
- [ ] Workflow actions qua API thật.
- [ ] List CTA ad-hoc đúng contract irregular.
- [ ] Build + tests pass.
- [ ] Manual E2E STEP 8 signed QA.

---

## 6. Tham chiếu

| Tài liệu | Path |
|----------|------|
| PO decisions | `deadline-alert-detail-po-decisions-summary.md` |
| Plan kiến trúc | `deadline-alert-detail-screen-implementation-plan.md` |
| Plan list/tab | `deadline-alerts-real-data-implementation-plan.md` |
| Pattern workflow | `cobo_web_design/src/pages/portal/DisclosureDetail.tsx` |
| Pattern publish | `cobo_web_design/src/pages/portal/DisclosureForm.tsx` |
| FE detail hiện tại | `cobo_web_design/src/pages/portal/DeadlineDetail.tsx` |

---

**Docs consulted:** `deadline-alert-detail-po-decisions-summary.md`, `DisclosureForm.tsx`, `DisclosureDetail.tsx`, `disclosureApi.ts`, `workflow/app/contracts.go`, `workflowOverrideMappers.ts`, `disclosure/transport/http/handler.go`.

**Cache:** `docs/ai-cache/deadline-alert-detail-phase5-execution-plan.md`
