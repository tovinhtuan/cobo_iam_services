# Deadline alert detail — STEP 2 foundation (summary)

**Date:** 2026-05-25  
**Scope:** `cobo_web_design` FE — Phase 5 detail screen foundation  
**Tickets:** FE-DA-D01, D02, D03, D04

## What shipped

1. **Workflow instance mapping**
   - `current_step_code` on `WorkflowInstanceDto`.
   - `mapSnapshotToWorkflowSteps` when BE returns `snapshot[]` without `workflow[]`.
   - `normalizeWorkflowInstanceDto` derives `current_step_index` from code or API index.

2. **View model**
   - `deadlineAlertDetailViewModels.ts`: step UI status, progress %, alert status from record/alert, degraded flag when instance id exists but no steps.

3. **DeadlineDetail load**
   - Parallel: disclosure, alerts list, workflow instance + tasks, disclosure type name.
   - Alert from `location.state.alert` or list match.
   - Timeline/progress from VM; no local Done/toggle mutations (PO §0 — STEP 4).

4. **DeadlineList navigation**
   - `Link` passes `state={{ alert: source }}` for faster first paint.

## Tests

- `deadlineAlertDetailViewModels.test.ts`
- `snapshotToWorkflowSteps.test.ts`
- `tsc --noEmit` clean

## Next

- **STEP 3:** `WorkflowStepsOverview`, `DeadlineWorkflowCard` from VM (remove 4-card mock).
- **STEP 4–5:** `actOnTask`, `PublishDisclosureModal` (evidence_link + submit).

## PO rule (unchanged)

Done on alert screen = workflow complete + disclosure published with evidence — no fake `complete alert` API.
