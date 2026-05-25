# Deadline alert detail — STEP 3 read-only UI (summary)

**Date:** 2026-05-25  
**Scope:** `cobo_web_design` — `FE-DA-D05`–`D08`, `D06`, `D18`

## Components

| File | Role |
|------|------|
| `pages/portal/deadlines/WorkflowStepsOverview.tsx` | Horizontal progress + N milestone cards |
| `pages/portal/deadlines/DeadlineWorkflowCard.tsx` | Per-step read-only card, tasks, documents |
| `pages/portal/deadlines/DeadlineAlertDetailSidebar.tsx` | Type, dates, status, notes placeholder |
| `pages/portal/DeadlineDetail.tsx` | Orchestration only |

## View model extensions

- Step VM: `documents`, `processingDays`, `isPublishStep`
- Helpers: `resolveStepDepartmentLabel`, `tasksForWorkflowStep`, `mergeStepDocuments`, `isPublishWorkflowStep`

## PO / D18

- Removed translation widget.
- Publish step: no email chips, no InfoBox grid; shows `evidenceLink` when present.
- Footer finish button disabled until STEP 5 publish modal.

## Tests

- `deadlineAlertDetailViewModels.test.ts` (extended)
- `WorkflowStepsOverview.test.tsx`

## Next

STEP 4: `actOnTask` from `DisclosureDetail` pattern.  
STEP 5: `PublishDisclosureModal` + enable finish controls.
