# 07 — Implementation plan — sequential unlock + alert button rename

**Depends on:** `06-step-sequential-unlock-solution.md`  
**Mode:** IMPLEMENTED locally (see `08-implementation-result.md`) — DEV smoke pending request

---

## 1. Ordered task IDs

| ID | Repo | Task | Depends |
|---|---|---|---|
| T0 | both | PO confirm Option A + Option 1; confirm early unlock applies to Step1 | — |
| T1 | BE | Unit tests (failing first): early unlock Step1→Step2 before planned_start; pred incomplete still locked; overdue still works; empty snapshot | T0 |
| T2 | BE | Change `resolveCurrentStepIndex` to sequential-first; adjust `classifyWorkflowStep` / `is_future` / `available_actions` gate | T1 |
| T3 | BE | Clarify `IsCurrentByTime` = calendar window ∩ today (independent of sequential current) | T2 |
| T4 | BE | Regression: timeliness + delay shifts + mark_incomplete blocking + complete idempotency | T2 |
| T5 | BE | Authz/tenant smoke via existing service tests; optional complete-path terminal freeze if gap found | T2 |
| T6 | FE | Update lock copy when sequential current but calendar early (if API exposes `is_current_by_time=false`) | T2 |
| T7 | FE | Rename button `Cập nhật cảnh báo` → `Chỉnh sửa tin công bố`; update any i18n keys/tests | T0 |
| T8 | FE | Vitest: status label / card gating with early-current fixture; navigation label test | T6,T7 |
| T9 | both | DEV smoke checklist (below); no Production | T4–T8 |
| T10 | both | ai-cache result note + READY_FOR_USER_COMMIT | T9 |

---

## 2. BE implementation notes (T2–T3)

### Proposed `resolveCurrentStepIndex` (conceptual)

```text
1. Keep: first incomplete with MarkedIncompleteAt → return i
2. NEW primary: first incomplete (CompletedAt == nil) → return i
3. Remove reliance on calendar window / overdue loops for selecting current
   (overdue becomes classification only via today vs planned_end)
```

### `classifyWorkflowStep`

- `isCurrent := index == currentIdx && !completed` (unchanged structure)
- For sequential current with `today.Before(planned_start)`: **must not** set `is_future=true`
- Later steps (`index > currentIdx`): remain `not_started` / `is_future=true` / locked

### Actions

```go
// Prefer: canManage && isCurrent && !isCompleted
// Drop redundant !isFuture once is_future cannot be true for current
```

### `IsCurrentByTime`

Today: `isCurrent && MarkedIncompleteAt == nil`.  
Target: `!today.Before(start) && !today.After(end) && !completed` (calendar), independent of sequential current.

### Files expected to change (BE)

```text
internal/deadlinealerts/app/steps_algorithm.go
internal/deadlinealerts/app/steps_algorithm_test.go
internal/deadlinealerts/app/timeliness_test.go  (if early-current fixtures needed)
```

Possibly untouched: handler, repo upsert (already correct), mysql schema.

### Files explicitly NOT to change

```text
migrations/*
workflowstepevidence/*
workflowstepcomments/*
disclosure publish APIs
rbac grant tables
```

---

## 3. FE implementation notes (T6–T8)

### Files expected to change (FE)

```text
src/pages/portal/DeadlineDetail.tsx          # button label
src/pages/portal/deadlines/DeadlineWorkflowCard.tsx  # lock copy if needed
src/i18n/* (if labels externalized)
src/services/deadlineStepsApi.test.ts
src/pages/portal/deadlines/*.test.tsx
```

### Files NOT to change

```text
Evidence/Discussion/History panels (behavior)
PortalLayout chrome
DisclosureForm fields/validation (rename destination only)
```

### Button Option 1 only

```tsx
// label only
Chỉnh sửa tin công bố
// keep navigate(`/app/disclosures/${record.id}/edit`)
```

---

## 4. API contract delta

Wire schema unchanged. Semantic changelog for consumers:

| Field | Before (early complete) | After Option A |
|---|---|---|
| `current_step_code` | often `""` | successor step code |
| Step2 `status` | `not_started` | `current` |
| Step2 `is_future` | `true` | `false` |
| Step2 `available_actions` | `[]` | `["complete", …]` if authz |
| `planned_*` | unchanged | unchanged |

Document in ai-cache after implement; no OpenAPI file required if project has none.

---

## 5. Migration decision

**DB_MIGRATION_REQUIRED=false** — no backfill; projection recomputed from existing `completed_at`.

---

## 6. Authz / tenant / audit / notification

| Area | Impact |
|---|---|
| Authz | Unchanged `deadline.confirm`; no rbac.manage bypass |
| Tenant | Unchanged company/record scope |
| Audit | Persist path unchanged; verify event still emitted |
| Notification | Planned dates unchanged; workers should not require calendar-current for step ops |

---

## 7. Test matrix

### Backend (T1/T4)

1. Step1 completed before Step2 planned_start → Step2 `current`, `available_actions` contains `complete`, `is_future=false`, planned dates unchanged.  
2. Step1 not completed → Step2 `is_future`/`locked`.  
3. Terminal record (if enforced on complete) → no complete.  
4. Marked incomplete blocking → stays current incomplete.  
5. Step2 completed → Step3 current.  
6. Complete replay / already completed → idempotent list.  
7. Unauthorized → 403.  
8. Cross-tenant → 403/404 scope.  
9. `current_step_code` deterministic non-empty while incomplete remain.  
10. Planned dates stable under early unlock.  
11. Timeliness not falsely overdue for early current.  
12. Existing B2/B3 evidence gates unchanged.

### Frontend (T8)

1. Fixture Step2 actionable → complete enabled.  
2. Fixture future locked → disabled + correct label.  
3. Complete response with Step2 current → UI updates without stale future.  
4. `available_actions` mapping.  
5. Lock reason / copy.  
6. Button label + still navigates to edit.  
7. Evidence/Discussion/History smoke (no behavior change).  
8. Terminal/disabled publish bar unchanged.

---

## 8. DEV smoke checklist (T9 — design only)

```text
1. Login tenant (non-CMS)
2. Open deadline ≥2 steps; note Step2 planned_start > today
3. Confirm Step1 actionable
4. Complete Step1 (card CTA) before Step2 start
5. Confirm response / reload: Step2 status=current, not “Chưa đến bước”
6. available_actions includes complete
7. Operate Step2 complete or evidence as allowed
8. Check audit row if tooling available
9. Click renamed button → DisclosureForm “Chỉnh sửa tin công bố”
10. Regression: published/done record actions frozen
11. Regression: Step3 still locked while Step2 current
```

---

## 9. Rollback

- Revert `steps_algorithm.go` (+ tests) and FE label/copy commits.  
- No migration to roll back.  
- Feature flag **not required** if change is small and tested; optional `DEADLINE_SEQUENTIAL_CURRENT=true` only if PO demands gated rollout.

---

## 10. Acceptance criteria

- [ ] Early complete Step N unlocks Step N+1 without waiting for planned_start.  
- [ ] Planned dates unchanged on unlock.  
- [ ] Later steps remain locked until predecessors complete.  
- [ ] FE shows Đang thực hiện / complete CTA, not Chưa đến bước, for early current.  
- [ ] Button label matches disclosure edit destination.  
- [ ] Focused BE/FE tests green; no Production deploy in first implement cycle unless PO asks.  
- [ ] CODE not staged/committed by agent unless user requests.

---

## 11. Definition of done

T0–T10 complete; solution docs updated with implement result note; `READY_FOR_USER_COMMIT=true`; STAGED/COMMITTED remain user-owned.

---

## 12. Risks

| Risk | Mitigation |
|---|---|
| Early Step1 start surprises ops | PO confirm in T0 |
| Timeliness edge cases | Explicit tests T4 |
| FE still shows time lock copy | T6 copy update |
| Confusing bottom “Kết thúc bước” | Separate ticket; document only |
