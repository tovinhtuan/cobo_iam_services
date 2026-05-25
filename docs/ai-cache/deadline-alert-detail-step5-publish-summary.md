# Deadline alert detail — STEP 5 publish (summary)

**Date:** 2026-05-25  
**Ticket:** FE-DA-D15 (publish)

## Flow (PO §0)

1. User completes workflow tasks (STEP 4).
2. Opens publish modal from footer or sidebar.
3. Enters `evidenceLink` (https) + confirms checklist.
4. `PATCH /disclosures/{id}` with evidence → `POST /disclosures/{id}/submit`.
5. Reload record + deadline-alerts list + workflow → alert `Done` when `Published`/`Completed`.

## Gating (`assessPublishReadiness`)

- `disclosure.publish` required.
- All tasks `confirmed` (no pending/reviewed/approved; no rejected).
- Record not already Published/Completed; alert not Done.

## Files

- `PublishDisclosureModal.tsx`
- `deadlinePublishReadiness.ts`
- `DeadlineDetail.tsx`, `DeadlineAlertDetailSidebar.tsx`

## Phase 5 FE core: complete (STEP 0–5)

Optional follow-ups: STEP 6 documents «+», BE notes (BE-DA-D12), E2E manual QA.
