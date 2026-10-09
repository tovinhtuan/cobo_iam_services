---
name: wf-bugfix
description: Workflow sửa bug Cobo có kỷ luật - thu thập triệu chứng, tái hiện bằng test fail trước, tìm root cause có bằng chứng, sửa nhỏ nhất, test hồi quy pass, đánh giá blast radius và dữ liệu hỏng, review theo vùng rủi ro. Dùng khi có bug, lỗi, regression, sự cố, hoặc lệnh /bugfix.
---

# Workflow: Bug Fix

Artifacts go to `docs/ai-cache/bug-<slug>-YYYY-MM-DD/` when the repo rules
require a task summary (non-trivial bugs).

## Phase 0 - Intake → `00-report.md`
Symptom, expected vs actual, environment, role/company, time, request id,
screenshots/console, frequency. Search ai-cache and `git log -S` for prior
work on the area.

## Phase 1 - Reproduce  **GATE R**
Follow `bug-triage-debug` steps 3-4. Produce the smallest reproduction:
1. Failing automated test (Go test / Vitest / contract test) - preferred.
2. Else a scripted repro (curl/Postman/E2E script) with exact steps.
**Do not change production code before a reproduction exists**, unless the
user accepts a fix based on code reading only; then state the risk.

## Phase 2 - Root cause → `01-root-cause.md`
One sentence, with evidence (file:line, log line, query result). Mark
confirmed vs suspected. Identify the layer (FE, API, service, DB, worker,
cache, config/flag, deploy).

## Phase 3 - Fix
Smallest safe change at the root cause, in the owning repo. Use the domain
skill for the layer (e.g. `backend-authz-session`, `deadline-reminder-holiday`,
`frontend-route-screen`). No refactors.

## Phase 4 - Verify  **GATE V**
- The reproduction test now passes and fails if the fix is reverted.
- Relevant baseline checks (see `cobo-task-workflow` Step 5).

## Phase 5 - Blast radius
- Other callers/paths with the same bug pattern (grep).
- Sibling repo contract affected?
- Data already corrupted → write a repair/backfill plan (do not run it
  without approval).

## Phase 6 - Review
If the fix touches auth, tenant scope, money/quota, migrations, worker or
cache: launch the matching reviewer subagents (path map in
`wf-risk-review`). Otherwise a self-review with `premerge-system-review`
(short form) is enough.

## Completion report
```text
Bug:
Reproduction (test name / script):
Root cause (evidence):
Fix (files):
Verification:
Blast radius / data repair:
Follow-ups:
```
