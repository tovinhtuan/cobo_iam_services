---
description: Rà soát rủi ro 6 nhóm (Security FE/BE, Performance, Cache, API compat, CORS & Credentials) bằng reviewer chạy song song
argument-hint: [diff [base] | all | <paths>]
---

Run the `wf-risk-review` skill, read-only.

Scope: $ARGUMENTS
(If empty, use `diff` against the merge-base with `main`.)

Launch the selected reviewer subagents in a single message so they run in
parallel, verify every CRITICAL/HIGH finding yourself, and write the report
to `docs/ai-cache/risk-review-<date>/10-risk-report.md`. Do not fix anything
until I ask.
