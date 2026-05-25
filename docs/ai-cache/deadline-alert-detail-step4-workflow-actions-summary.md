# Deadline alert detail — STEP 4 workflow actions (summary)

**Date:** 2026-05-25  
**Ticket:** FE-DA-D15 (task actions)

## Behavior

- Permissions: `workflow.review` → pending; `workflow.approve` → reviewed; `workflow.confirm` → approved.
- `actOnTask` then `reloadWorkflow` (instance + tasks) → timeline/cards refresh from VM.
- Reject prompts for comment; errors via `formatWorkflowApiErrorMessage`.

## Files

| File | Change |
|------|--------|
| `services/workflowTaskActions.ts` | Shared action resolver + VI labels |
| `services/workflowTaskActions.test.ts` | Permission matrix tests |
| `pages/portal/DeadlineDetail.tsx` | Handlers + reload + banners |
| `pages/portal/deadlines/DeadlineWorkflowCard.tsx` | Action buttons per task |

## Next

STEP 5: `PublishDisclosureModal`, enable footer/sidebar finish when published with evidence.
