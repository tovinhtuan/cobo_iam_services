# Batch 3 — FE deadline detail + preferences

**Date:** 2026-05-27  
**Contract:** business-contract-workflow-deadline-final.md v1.1 (AC-4–6, §5–6.3)

## FE behavior

- `workflowDegraded` only when no `workflow_instance_id` (OQ-DA-03).
- `workflowSnapshotError` when instance id exists but steps empty (WF2-A); not degraded.
- `pending_init` synthetic step + banners in detail/overview/cards.
- `deriveAlertStatus`: Published/Completed → `Pending Confirm` without BE confirm (DC-B).
- Preferences tab: GET `disclosure.view`, PATCH `disclosure.auto_create.manage` only (no view required for PATCH).

## Verify

```bash
cd cobo_web_design
npm run test -- src/services/deadlineAlertDetailViewModels.test.ts src/services/deadlineAlertsApi.test.ts src/services/disclosureTypesApi.contract.test.ts
```
