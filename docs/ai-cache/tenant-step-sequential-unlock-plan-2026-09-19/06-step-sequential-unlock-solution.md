# 06 — Step sequential unlock + “Cập nhật cảnh báo” navigation — Solution

**Date:** 2026-09-19  
**Repos:** cobo_iam_services + cobo_web_design  
**Mode:** analysis / design only — no code, migration, deploy, or commit

---

## 1. Executive summary

Two independent Tenant Portal defects share the Deadline Detail surface:

| # | Symptom | Root layer | Recommended fix |
|---|---|---|---|
| 1 | After Step 1 complete (before Step 2 planned start), Step 2 shows **Chưa đến bước**, locked, no `complete` | **BE** `resolveCurrentStepIndex` is calendar-window-only; empty `current_step_code` → FE labels from `is_future` | **Option A** — sequential eligibility (first incomplete step = current); keep planned dates for SLA/timeline only |
| 2 | Button **Cập nhật cảnh báo** opens disclosure edit titled **Chỉnh sửa tin công bố** | **FE** intentional `navigate(/app/disclosures/:id/edit)` | **Option 1** — rename button to match destination (no new API) |

FE already applies BE `available_actions` / `is_future` faithfully and reloads steps from complete response. No evidence of FE stale state as primary unlock bug.

---

## 2. Evidence — Part A source trace (workflow)

### A1. Frontend

| Symbol | Evidence |
|---|---|
| `FE_DEADLINE_DETAIL_COMPONENT` | `cobo_web_design/src/pages/portal/DeadlineDetail.tsx` |
| `FE_STEP_CARD_COMPONENT` | `…/deadlines/DeadlineWorkflowCard.tsx` (+ panel `DeadlineCurrentStepPanel.tsx`) |
| `FE_COMPLETE_STEP_HANDLER` | `DeadlineDetail.handleCompleteStep` → `deadlineStepsApi.completeStep` (~L658–702) |
| `FE_STEP_ACTION_GATING` | Card: `available_actions.includes('complete')` before wiring `onCompleteStep` (DeadlineDetail ~L1056–1058; card `canComplete` ~L408) |
| `FE_STEP_STATUS_MAPPER` | `deadlineStepStatusLabel` in `src/services/deadlineStepsApi.ts` (~L208–223) |
| `FE_LOCK_MESSAGE_SOURCE` | `DeadlineWorkflowCard` ~L453–456: copy **“Bị khóa đến khi tới thời gian thực hiện”** when `isLocked && isFuture` |
| `FE_CURRENT_STEP_SOURCE` | BE `current_step_code` + per-step `status === 'current'`; overview via `deadlineWorkflowProgress` / operational VM |
| `FE_RELOAD_AFTER_COMPLETE` | `setApiSteps(res.data)` from complete API body — **no separate GET required**; also `reloadSteps` on full refresh |

**Two different “Kết thúc bước” controls (must not conflate):**

| Control | Location | Semantics |
|---|---|---|
| Step complete | `DeadlineWorkflowCard` when `available_actions` contains `complete` | `POST …/steps/{step_code}/complete` |
| Bottom bar primary | Fixed dock label **Kết thúc bước** / **Xác nhận hoàn tất** | Publish modal or deadline confirm — **not** step complete (~L1135–1166) |

### A2. Backend

| Symbol | Evidence |
|---|---|
| `BE_STEPS_ENDPOINT` | `GET /api/v1/company/deadlines/{record_id}/steps` — `handler.go` |
| `BE_COMPLETE_ENDPOINT` | `POST …/steps/{step_code}/complete` — `handler.go` |
| `BE_HANDLER` | `internal/deadlinealerts/transport/http/handler.go` (`listDeadlineSteps`, `completeDeadlineStep`) |
| `BE_SERVICE` | `internal/deadlinealerts/app/steps_service.go` (`ListDeadlineSteps`, `CompleteDeadlineStep`) |
| `BE_STATE_REPOSITORY` | `internal/deadlinealerts/infra/mysql/steps_repository.go` (`ListStepStates`, `UpsertStepCompleted`) |
| `BE_ALGORITHM` | `internal/deadlinealerts/app/steps_algorithm.go` (`ComputeDeadlineSteps`, `resolveCurrentStepIndex`, `classifyWorkflowStep`) |
| `BE_AUTHZ` | `canManageDeadlineSteps` → Authorize action `deadline.confirm` (not `rbac.manage` bypass on steps) |
| `BE_AUDIT` | Completion persist via upsert; dedicated step-complete audit events — **confirm in implement** against existing audit emitters (may already be on fulfillment path) |
| `BE_EXISTING_TESTS` | `steps_algorithm_test.go`, `timeliness_test.go`, handler stubs; **no** test for “Step1 completed before Step2 planned_start → Step2 current” |

Also: `workflowfulfillment/deadline_bridge.go` calls `ComputeDeadlineSteps` for list/card projections — same algorithm.

### A3. Root cause — step unlock (evidence-backed)

#### Does complete persist `completed_at`?

**Yes.** `CompleteDeadlineStep` → `UpsertStepCompleted` writes `workflow_instance_step_states.completed_at` then returns `ListDeadlineSteps` with fresh states.

#### Does GET/list after complete read new state?

**Yes.** Complete returns `ListDeadlineSteps`; FE `setApiSteps(res.data)`.

#### How does `resolveCurrentStepIndex` choose current?

```text
1) first non-completed with MarkedIncompleteAt → blocking
2) first non-completed whose today ∈ [planned_start, planned_end]
3) first non-completed with today > planned_end (overdue)
4) else -1
```

**No branch:** “first non-completed after predecessors completed” independent of calendar.

#### Early-complete scenario (product bug)

```text
Step1 completed_at set
today < Step2.planned_start
→ windows skip Step2
→ overdue skip Step2
→ currentIdx = -1
→ current_step_code = ""
→ classify Step2: today.Before(start) ⇒ status=not_started, is_future=true, is_locked=true
→ available_actions=[] (requires isCurrent && !isFuture)
```

FE then shows `deadlineStepStatusLabel('not_started', true)` → **Chưa đến bước** and lock copy about time — correct relative to API.

#### Is the bug FE mapper?

**No (primary).** Mapper/labels mirror BE flags. Secondary UX debt: FE hardcodes time-based lock copy even if product later unlocks early.

#### `current_step_code` empty?

**Yes, possible** when no step’s calendar window matches and none overdue — proven by algorithm when all remaining steps are strictly future.

#### Existing tests gap

`TestComputeDeadlineSteps_Step2Current_Step1Completed` uses `today` **inside** Step2 window (2026-07-06). It does **not** cover early unlock before Step2 start.

---

## 3. Root cause — “Cập nhật cảnh báo” navigation

| Field | Evidence |
|---|---|
| `UPDATE_ALERT_BUTTON_COMPONENT` | `DeadlineDetail.tsx` fixed action bar (~L1120–1126) |
| `UPDATE_ALERT_BUTTON_HANDLER` | `onClick={() => navigate(\`/app/disclosures/${record.id}/edit\`)}` |
| `UPDATE_ALERT_DESTINATION` | React Router navigate to edit |
| `DESTINATION_ROUTE` | `/app/disclosures/:id/edit` (`App.tsx`) |
| `DESTINATION_COMPONENT` | `DisclosureForm` with `isEdit` |
| `DESTINATION_TITLE` | H1 **Chỉnh sửa tin công bố** (`DisclosureForm.tsx` ~L401) |

**Not a wrong route bug** — label/product naming mismatch. No deadline-alert PATCH resource found for “update alert only” on this surface → Option 2 blocked without new BE contract.

---

## 4. Product contract recommendation

### Step unlock — **Option A (recommended)**

```text
current_step := first non-completed step in order
  (respect marked_incomplete blocking)
  (independent of planned_start)
```

Planned start/end remain for SLA, timeline display, overdue/early timeliness, reminders.

**Reject Option B** (calendar AND predecessor) unless PO explicitly wants early operators blocked — conflicts with stated product desire.

### Actionable when (Option A)

```text
- step is sequential current (first incomplete)
- not completed
- record not terminal (list/alert status: completed|done|published — already used for alert cards; enforce on complete path if missing)
- not blocked by incomplete policy on earlier/current incomplete mark
- authorize deadline.confirm (existing)
```

### Alert button — **Option 1 (recommended)**

Rename label → **Chỉnh sửa tin công bố**. Keep route. Defer Option 2 until alert-config API exists.

---

## 5. State machine (Option A)

> **PO lock (2026-09-19):** `EARLY_START_FIRST_STEP=false` — Step 1 remains locked until `planned_start`.
> `SUCCESSOR_UNLOCK_AFTER_PREDECESSOR_COMPLETION=true` — Step N+1 opens immediately after Step N completes.
> `current_step_code` may be empty when Step 1 has not reached `planned_start` yet.

| Current situation | Condition | Next projection | Actionable |
|---|---|---|---|
| Step1 before planned_start | EARLY_START_FIRST_STEP=false | no current; Step1 not_started/locked | No |
| Step1 current | complete OK | Step1 completed; Step2 status=current | Step2 yes (if authz) |
| Step2 not_started, pred incomplete | pred not completed | stay future/locked | No |
| Step2 calendar future, pred completed | sequential unlock | current; is_future=false | Yes |
| Step2 current | authz OK | current | Yes |
| Step2 end < today, incomplete | overdue classification | overdue/current policy | Yes if sequential current |
| Record terminal | published/done/completed | freeze mutations | No |
| Marked incomplete (blocking) | MarkedIncompleteAt set | incomplete current | mark_incomplete policy |

### Token semantics (implemented)

| Token | Meaning after Option A |
|---|---|
| `current_step_code` | Sequential current; empty only when first step not yet started |
| `status=current` | Sequential current |
| `is_current_by_time` | Sequential current ∩ calendar window (false on early unlock) |
| `is_future` | Strictly after sequential current (later steps), **not** “before my planned_start while I am sequential current” |
| `is_locked` | completed OR not sequential current OR policy lock |
| `available_actions` | `complete` / `mark_incomplete` when canManage && sequential current && !completed |
| `lock_reason` | `completed` \| `not_started` (later steps) \| `not_current` \| `past_incomplete` |
| planned_* | Unchanged by unlock |

**Breaking risk:** clients that treated `is_future` as pure calendar may need FE copy update (DeadlineWorkflowCard lock string — done).

---

## 6. Cross-repo impact (Part D)

| Flag | Value | Notes |
|---|---|---|
| `API_CONTRACT_CHANGE_REQUIRED` | **soft true** | Same fields; semantic change of `is_future` / `current_step_code` / `available_actions` for early-complete cases |
| `DB_MIGRATION_REQUIRED` | **false** | Projection-only; states table already stores `completed_at` |
| `BE_ONLY_CHANGE` | false | Algorithm BE-primary; FE copy/button rename |
| `FE_ONLY_CHANGE` | false | |
| `CROSS_REPO_CHANGE` | **true** | |
| `AUDIT_CHANGE_REQUIRED` | **false** (expected) | Persist path unchanged; verify emitters |
| `NOTIFICATION_CHANGE_REQUIRED` | **false** (expected) | Reminder windows still use planned dates; confirm no worker assumes calendar-only current |

Authz: keep `deadline.confirm`; do **not** add `rbac.manage` bypass.  
Tenant: unchanged company/record scope in `loadWorkflowForRecord`.

---

## 7. Open decisions / risks

1. **PO:** Confirm Option A early start for Step1 itself when today < Step1.planned_start (algorithm change makes first incomplete current immediately).
2. **Timeliness:** Early current with `IsFuture=false` may yield early/on_time markers — add regression tests in `timeliness_test.go`.
3. **Bottom bar label “Kết thúc bước”** for publish — separate UX debt; do not mix with step complete in implementation.
4. **List card `CurrentStepNameFromTimelines`** uses same algorithm — will pick up Option A automatically (good).
5. **Audit completeness** for step complete — verify during implement (UNKNOWN until emitter grep in implement task).

---

## 8. Explicit non-goals

- No production deploy in plan phase.
- No new disclosure/alert edit API (Option 2).
- No change to Evidence/Discussion/History contracts.
- No calendar date mutation on early unlock.
