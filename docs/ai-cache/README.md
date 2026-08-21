## Recurring disclosure Effective T Scheduling V1 (2026-08-21)

- Pointer: FE `../cobo_web_design/docs/ai-cache/recurring-disclosure-effective-t-2026-08-21/`
- BE: effective schedule resolver, worker seed, clear override, submitted_at/open_at migration 0131
- Verdict: implemented + DEV deploy; wait confirmation
- NO_PRODUCTION / NO_COMMIT / NO_PUSH

## Recurring disclosure Effective T — reconciliation STOP (2026-08-21)

- Pointer: FE `../cobo_web_design/docs/ai-cache/recurring-disclosure-effective-t-2026-08-21/`
- Verdict: wait PO (inclusive vs exclusive Due, cutoff, legacy T relabel); no implement yet
- NO_CODE / NO_DB / NO_DEPLOY / NO_COMMIT / NO_PUSH

## Recurring disclosure business contract extraction (2026-08-21)

- Pointer: FE `../cobo_web_design/docs/ai-cache/recurring-disclosure-business-contract-extraction-2026-08-21.md`
- Verdict: `RECURRING_DISCLOSURE_BUSINESS_CONTRACT_EXTRACTION_COMPLETE` — wait PO; `IMPLEMENTATION_SAFE_TO_START=false`
- NO_CODE / NO_DB / NO_DEPLOY / NO_COMMIT / NO_PUSH

## Template cycle / periodicity vs deadline — source audit (2026-08-21)

- Pointer: FE `../cobo_web_design/docs/ai-cache/template-cycle-deadline-source-audit-2026-08-21.md`
- Verdict `TEMPLATE_CYCLE_DEADLINE_SOURCE_AUDIT_COMPLETE` / superseded for scheduling by recurring-disclosure extraction
- NO_CODE / NO_DB / NO_DEPLOY / NO_COMMIT / NO_PUSH

## Workflow publish readiness + Đăng lên Portal CTA (2026-08-21)

- Pointer: FE `../cobo_web_design/docs/ai-cache/workflow-publish-readiness-fix-2026-08-21/`
- BE: `activation_ready` / blockers on version GET; Activate code split empty vs invalid
- Verdict: `WORKFLOW_PUBLISH_TO_PORTAL_DEV_VERIFIED`
- NO_PRODUCTION / NO_COMMIT / NO_PUSH

## Template portal state filter (2026-08-21)

- Pointer: FE `../cobo_web_design/docs/ai-cache/template-portal-state-filter-2026-08-21/`
- Verdict `TEMPLATE_PORTAL_STATE_FILTER_DEV_VERIFIED`
- NO_PRODUCTION / NO_COMMIT / NO_PUSH

## Template status filter source audit (2026-08-21)

- Pointer: FE `../cobo_web_design/docs/ai-cache/template-status-filter-source-audit-2026-08-21.md`
- Verdict `TEMPLATE_STATUS_SOURCE_AUDIT_COMPLETE` / wait PO
- NO_CODE / NO_DB / NO_DEPLOY / NO_COMMIT / NO_PUSH

## Template clone / create-from-existing — analysis plan (2026-08-21)

- Pointer: FE `docs/ai-cache/template-clone-create-from-existing-analysis-2026-08-21/`
- Verdict `TEMPLATE_CLONE_CREATE_FROM_EXISTING_PLAN_READY` (analysis only; no BE product code this run)
- NO_CODE / NO_DB / NO_DEPLOY / NO_COMMIT / NO_PUSH

## Company workflow override contract (2026-08-20)

- COMPANY_OVERRIDE_ACTIVE > ResolveCMSDefaultWorkflow; draft/activate isolation; reset fallback
- Evidence (FE pack): ../cobo_web_design/docs/ai-cache/company-workflow-override-contract-2026-08-20/
- Verdict `COMPANY_WORKFLOW_OVERRIDE_CONTRACT_DEV_VERIFIED`
- NO_PRODUCTION / NO_PUSH

﻿## Phase 1 — Centralize Workflow Authority (2026-08-20)

- Pointer: FE `docs/ai-cache/phase1-centralize-workflow-authority-2026-08-20/`
- BE: `ResolveCMSDefaultWorkflow` + Activate/GetEffective adapters; `GetActiveGlobalWorkflow`
- Verdict `PHASE1_CENTRALIZED_WORKFLOW_AUTHORITY_DEV_VERIFIED`; api/worker `2026-08-20T07:29:19Z`
- NO_PRODUCTION / NO_PUSH

## Deadline alert visibility — active template only (2026-08-18)

- Pointer: FE `docs/ai-cache/deadline-alert-active-template-filter-2026-08-18/`
- ListRows INNER JOIN current `active_version_no > 0`; no business delete
- Verdict `DEV_DEADLINE_ALERT_ACTIVE_TEMPLATE_FILTER_VERIFIED`; api `2026-08-18T08:47:17Z`
- NO_PRODUCTION / NO_PUSH

## Deadline alert cleanup before 2026-08-17 (DEV, blocked)

- Pointer: FE `docs/ai-cache/deadline-alert-cleanup-before-2026-08-17/`
- Entity = `disclosure_records`; no delete executed
- `BLOCKED_DEADLINE_ALERT_ENTITY_IS_BUSINESS_RECORD`
- NO_PRODUCTION / NO_PUSH

## Workflow step reminder — email business context (2026-08-18)

- Pointer: FE `docs/ai-cache/workflow-step-reminder-rule-engine-2026-08-17/email-business-context-audit-2026-08-18/`
- Reminder CTA/deadline fix; company source = proposal/instance; api+worker `2026-08-18T05:12:07Z`
- `BLOCKED_EMAIL_EVIDENCE_UNAVAILABLE=CLOSED`; 2 paused blockers unchanged
- NO_PRODUCTION / NO_PUSH

## Workflow step reminder — E1 no-head fallback (2026-08-18)

- Pointer: FE `docs/ai-cache/workflow-step-reminder-rule-engine-2026-08-17/e1-no-head-fallback-2026-08-18/`
- Verdict `WORKFLOW_ALERT_E1_NO_HEAD_FALLBACK_DEV_VERIFIED`; api+worker `2026-08-18T03:46:59Z`
- NOT `WORKFLOW_STEP_REMINDER_RULE_DEEP_SMOKE_DEV_READY`; 3 paused blockers unchanged
- NO_PRODUCTION / NO_PUSH

## Workflow step reminder — recipient authority A/B/C/D (2026-08-18)

- Pointer: FE `docs/ai-cache/workflow-step-reminder-rule-engine-2026-08-17/recipient-authority-verify-2026-08-18/`
- Verdict `WORKFLOW_ALERT_RECIPIENT_AUTHORITY_DEV_VERIFIED`; api+worker `2026-08-18T02:40:45Z`
- NOT `WORKFLOW_STEP_REMINDER_RULE_DEEP_SMOKE_DEV_READY`; 3 paused blockers unchanged
- NO_PRODUCTION / NO_PUSH

## Workflow step reminder — department alert Step 1 (2026-08-18)

- Pointer: FE `docs/ai-cache/workflow-step-reminder-rule-engine-2026-08-17/department-alert-validation-2026-08-18/`
- Step 1 PARTIAL (mailbox Level 3 missing); snapshot-first recipient fix uncommitted; DEV api+worker `2026-08-18T01:33:28Z`
- Plan only: `19-three-blocker-resolution-plan.md`
- NOT `WORKFLOW_STEP_REMINDER_RULE_DEEP_SMOKE_DEV_READY`
- NO_PRODUCTION / NO_PUSH

## Workflow step reminder — migration recovery (2026-08-17)

- Pointer: FE pack `workflow-step-reminder-rule-engine-2026-08-17/` (`76`–`85` + `dev-smoke-custom-default/55+`)
- 0129 applied; `MIGRATION_REQUIRED=true` / `MIGRATION_IMPLEMENTED=true`; source-ready restored
- Remaining: `BLOCKED_EMAIL_EVIDENCE_UNAVAILABLE` (SMTP Gmail, not Mailpit)
- NOT `WORKFLOW_STEP_REMINDER_RULE_DEEP_SMOKE_DEV_READY`
- NO_PRODUCTION / NO_PUSH

## Workflow step reminder — Phase 3–5 DEV (2026-08-17)

- Pointer: FE pack `cobo_web_design/docs/ai-cache/workflow-step-reminder-rule-engine-2026-08-17/dev-smoke-custom-default/`
- Phase 3–4 PASS; Phase 5 ENUM blocker repaired by 0129 (see recovery entry)
- NOT `WORKFLOW_STEP_REMINDER_RULE_DEEP_SMOKE_DEV_READY`
- NO_PRODUCTION / NO_PUSH

## Workflow step reminder — Phase 2D source-ready (2026-08-17)

- Pointer: FE pack `cobo_web_design/docs/ai-cache/workflow-step-reminder-rule-engine-2026-08-17/` (`62`–`75`)
- Verdict **WORKFLOW_STEP_REMINDER_RULE_ENGINE_READY**
- Docker: `build api` + `build worker` + FE `run --rm --no-deps web npm ci && npm run build`
- NO_MIGRATION / NO_DEPLOY / NO_PRODUCTION / NO_PUSH; Phase 3 awaits confirm

## Workflow step reminder — Phase 2C BE runtime (2026-08-17)

- Pointer: FE pack `cobo_web_design/docs/ai-cache/workflow-step-reminder-rule-engine-2026-08-17/` (`46`–`61`)
- Verdict **WORKFLOW_STEP_REMINDER_PHASE2C_RUNTIME_ENGINE_READY**
- BE product: resolver + Path B `documents_json` persist + snapshot + due-minus engine
- NO_MIGRATION / NO_DEPLOY / NO_PRODUCTION / NO_PUSH; Phase 2D awaits confirm

## Workflow step reminder — Phase 2B CMS FE (2026-08-17)

- Pointer: FE pack `cobo_web_design/docs/ai-cache/workflow-step-reminder-rule-engine-2026-08-17/` (`28`–`29`, `37`–`45`)
- Verdict **WORKFLOW_STEP_REMINDER_PHASE2B_CMS_FE_READY**
- BE product source **unchanged**; runtime authority still planned `internal/disclosure/app/workflow_step_reminder_rule.go` (Phase 2C)
- NO_BACKEND_SOURCE_CHANGE / NO_MIGRATION / NO_DEPLOY / NO_PRODUCTION / NO_PUSH

## Workflow step reminder — Phase 2A contract lock (2026-08-17)


- Pointer: FE pack `cobo_web_design/docs/ai-cache/workflow-step-reminder-rule-engine-2026-08-17/` (`24`–`27`)
- Verdict **WORKFLOW_STEP_REMINDER_PHASE2A_CONTRACT_LOCKED**
- BE: no product source this phase; planned runtime authority `internal/disclosure/app/workflow_step_reminder_rule.go` (Phase 2C)
- NO_BACKEND_SOURCE_CHANGE / NO_MIGRATION / NO_DEPLOY / NO_PRODUCTION / NO_PUSH

## CMS irregular → ad_hoc template persistence fix (2026-08-17)

- Pointer: FE pack `cobo_web_design/docs/ai-cache/cms-irregular-adhoc-template-persistence-fix-2026-08-17/`
- Verdict **CMS_IRREGULAR_ADHOC_TEMPLATE_PERSISTENCE_FIX_READY** (FE-only)
- BE: no source/DTO/validation/alert-engine change required
- NO_BACKEND_SOURCE_CHANGE / NO_MIGRATION / NO_DEV_DEPLOY / NO_PRODUCTION / NO_PUSH



## CMS template → Enterprise abnormal alert post-fix deep smoke (2026-08-17)

- Pointer: FE pack `cobo_web_design/docs/ai-cache/cms-template-abnormal-alert-deep-smoke-2026-08-17/post-fix/`
- Verdict **CMS_TEMPLATE_TO_ENTERPRISE_ABNORMAL_ALERT_DEEP_SMOKE_DEV_READY**
- BE unchanged this closeout; API/worker StartedAt `2026-08-14T03:39:45Z`
- NO_BACKEND_DEPLOY / NO_WORKER_RESTART / NO_MIGRATION / NO_PRODUCTION / NO_PUSH

## Manual QR company package activation DEV (2026-08-14)

- Verdict **COBO_MANUAL_QR_COMPANY_PACKAGE_ACTIVATION_DEV_READY**
- `make deploy-be` recreates api+worker; no migrate; live lookup `SELECT company_code`
- Evidence (FE pack): `cobo_web_design/docs/ai-cache/manual-qr-company-package-activation-2026-08-14/` (`24`–`65`)
- NO_MIGRATION / NO_PRODUCTION / NO_PUSH

## Manual QR company package activation (2026-08-14)

- Verdict **COBO_MANUAL_QR_COMPANY_PACKAGE_ACTIVATION_READY**
- `ActivateImmediate` + CMS POST subscription/activate; origin `platform_admin_manual`
- Evidence (FE pack): `cobo_web_design/docs/ai-cache/manual-qr-company-package-activation-2026-08-14/`
- NO_MIGRATION / NO_WORKER / NO_DEV_DEPLOY / NO_PRODUCTION / NO_PUSH

## Manual QR package payment flow analysis (2026-08-14)

- Verdict **COBO_MANUAL_QR_PACKAGE_PAYMENT_FLOW_ANALYSIS_READY**
- Evidence (FE pack): `cobo_web_design/docs/ai-cache/package-manual-qr-payment-flow-analysis-2026-08-14/`
- Activation SoT: `companyplan.Writer.Create` / `company_subscriptions`; no HTTP yet
- NO_SOURCE_IMPLEMENTATION / NO_DEV_DEPLOY / NO_PRODUCTION / NO_PUSH

## CMS — template daily/weekly cycle DEV (2026-08-14)

- Verdict **CMS_TEMPLATE_DAILY_WEEKLY_CYCLE_DEV_READY**
- `make deploy-be` api+worker; worker SQL includes daily/weekly; PERIODIC_SEEDING_ENABLED=false
- Evidence (FE pack): `cobo_web_design/docs/ai-cache/cms-template-daily-weekly-cycle-2026-08-14/`
- NO_MIGRATION / NO_PRODUCTION / NO_PUSH

## Ad-hoc proposal tracking — T5 DEV deploy + E2E (2026-08-10)


- Verdict **ADHOC_PROPOSAL_TRACKING_DEV_READY**
- BE T1+T3 + FE T2+T3 deployed DEV; hotfix: propose-only detail no longer hard-redirects on admin org-directory 403
- Evidence: `adhoc-proposal-tracking-discoverability-2026-08-10/` (`113`–`148`)
- NO_PRODUCTION — STOP at DEV
## Ad-hoc proposal tracking â€” T4 integration (2026-08-10)

- Verdict **T4_ADHOC_PROPOSAL_TRACKING_INTEGRATION_READY**
- Docker API build parity **PASS**; worker redeploy not required for tracking
- Evidence (FE pack): `cobo_web_design/docs/ai-cache/adhoc-proposal-tracking-discoverability-2026-08-10/` (`91`â€“`112`)
- No migration / no deploy â€” await T5 confirm

## Ad-hoc proposal tracking â€” T3 detail runtime (2026-08-10)

- Verdict **T3_ADHOC_PROPOSAL_TRACKING_DETAIL_READY**
- Mode **T3_BE_DETAIL_PROJECTION_ADDED**: `tracking` on GetProposal via workflow instance/tasks
- Evidence (FE pack): `cobo_web_design/docs/ai-cache/adhoc-proposal-tracking-discoverability-2026-08-10/` (`72`â€“`90`)
- Quality: adhoc tests + vet + api build PASS; docker api build **BLOCKED** (daemon)
- No migration / no deploy â€” await T4/T5 confirm

## Ad-hoc proposal tracking â€” T1 backend (2026-08-10)

- Verdict **T1_ADHOC_PROPOSAL_TRACKING_BACKEND_READY**
- Self-detail + `scope=my`; company-wide list still `ad_hoc_alert.read`
- Evidence (FE pack): `cobo_web_design/docs/ai-cache/adhoc-proposal-tracking-discoverability-2026-08-10/` (`44`â€“`56`)
- Quality: adhoc tests + vet + api build + docker api build PASS
- No migration / no FE / no deploy â€” await T2 confirm

## Ad-hoc proposal tracking â€” T0 contract (2026-08-10)

- Verdict **T0_ADHOC_PROPOSAL_TRACKING_PRODUCT_CONTRACT_READY**
- **`T0_BACKEND_SELF_READ_REQUIRED=true`**: creator self-detail + `scope=my`; company-wide list still `ad_hoc_alert.read`
- Evidence (FE pack): `cobo_web_design/docs/ai-cache/adhoc-proposal-tracking-discoverability-2026-08-10/` (`30`â€“`43`)
- No BE source/migration/deploy â€” await confirm â†’ T1

## Ad-hoc proposal tracking discoverability â€” plan (2026-08-10)

- Verdict **ADHOC_PROPOSAL_TRACKING_IMPLEMENTATION_PLAN_READY** (FE evidence pack)
- List/Get APIs already exist; primary gap is FE IA/nav + optional authz for propose-only creators
- Evidence (FE): `cobo_web_design/docs/ai-cache/adhoc-proposal-tracking-discoverability-2026-08-10/`
- No BE source/migration/deploy â€” await confirm

## Ad-hoc proposal multi-assignee â€” M4 DEV (2026-08-10)

- Verdict **ADHOC_PROPOSAL_MULTI_ASSIGNEE_DEV_READY**
- Migration `0128` applied DEV only; BE api+worker + FE deployed; authenticated E2E PASS
- Evidence: `docs/ai-cache/adhoc-proposal-multi-assignee-2026-08-10/` (`93`â€“`130`) + results.m4
- Rollback: requires v3 task drain/compat plan (not blind app rollback)
- NO_PRODUCTION â€” STOP at DEV

## Ad-hoc proposal multi-assignee â€” M3 FE (2026-08-10)

- Verdict **M3_ADHOC_PROPOSAL_MULTI_ASSIGNEE_FRONTEND_READY** (FE pointer)
- BE source unchanged this phase; evidence pack `76`â€“`92` on FE (+ results.m3)
- Next: M4 migration apply + coordinated deploy

## Ad-hoc proposal multi-assignee â€” M2 runtime + recipients (2026-08-10)

- Verdict **M2_ADHOC_PROPOSAL_MULTI_ASSIGNEE_RUNTIME_READY**
- Migration `0128` source only (nullable singular + workflow_task_assignees); NOT applied
- Runtime: v3 one logical task + relation; ANY completion; Personal Ops / deadlinealerts / reminder readers model-aware
- Evidence: `docs/ai-cache/adhoc-proposal-multi-assignee-2026-08-10/` (`56`â€“`75`) + FE mirror pack
- No FE / no DEV deploy / no Production â€” await M3 confirm

## Ad-hoc proposal multi-assignee â€” M1 backend contract (2026-08-10)

- Verdict **M1_ADHOC_PROPOSAL_MULTI_ASSIGNEE_BACKEND_CONTRACT_READY**
- Source in this repo: workflow v3 snapshot + head resolver + submit normalize; v3 materialize guarded
- Canonical evidence: `../cobo_web_design/docs/ai-cache/adhoc-proposal-multi-assignee-2026-08-10/` (`40`â€“`55`)
- No migration / no DEV deploy / no Production â€” await M2 confirm

## Ad-hoc proposal multi-assignee â€” M0 Product contract lock (2026-08-10)

- Verdict **M0_ADHOC_PROPOSAL_MULTI_ASSIGNEE_PRODUCT_CONTRACT_READY** (pointer)
- Canonical evidence: `../cobo_web_design/docs/ai-cache/adhoc-proposal-multi-assignee-2026-08-10/` (`28`â€“`39`)
- Locked: ANY; schema_version=3; workflow_task_assignees; submit-time head; active-step multi recipients
- Superseded implement start â†’ M1 (see above)

## Ad-hoc proposal multi-assignee + department-head default â€” audit/plan (2026-08-10)

- Verdict **ADHOC_PROPOSAL_MULTI_ASSIGNEE_PLAN_BLOCKED_PRODUCT_DECISION** (historical pointer)
- Canonical evidence: `../cobo_web_design/docs/ai-cache/adhoc-proposal-multi-assignee-2026-08-10/`
- Docs-only; blocker cleared by M0
- Alert historical: **NO_CURRENTLY_SINGLE_RECIPIENT** (task/inbox singular)

## Ad-hoc proposal deadline day type â€” Phase D DEV (2026-08-10)

- Verdict **ADHOC_PROPOSAL_DEADLINE_DAY_TYPE_DEV_READY**
- Migration `0127` applied on DEV; api+worker+fe deployed; canonical evidence in cobo_web_design pack `60`â€“`90`
- NO_PRODUCTION

## Ad-hoc proposal deadline day type â€” Phase C runtime (2026-08-10)

- Verdict **PHASE_C_ADHOC_PROPOSAL_DEADLINE_DAY_TYPE_RUNTIME_READY**
- Source: `FormatProposalDueDate` + `deadlineengine.AddDaysAfter`; alerts/personalops wired
- Canonical evidence: `../cobo_web_design/docs/ai-cache/adhoc-proposal-deadline-day-type-2026-08-10/` (`44`â€“`59`)
- Migration 0127 not applied; no DEV/Production deploy â€” await Phase D

## Ad-hoc proposal deadline day type â€” Phase B FE (2026-08-10)

- Verdict **PHASE_B_ADHOC_PROPOSAL_DEADLINE_DAY_TYPE_FRONTEND_READY** (pointer)
- Canonical evidence: `../cobo_web_design/docs/ai-cache/adhoc-proposal-deadline-day-type-2026-08-10/`
- No BE source this phase; no deploy

## Ad-hoc proposal deadline day type â€” Phase A (2026-08-10)

- Verdict **PHASE_A_ADHOC_PROPOSAL_DEADLINE_DAY_TYPE_BACKEND_READY**
- Source: `proposed_deadline_day_type` + migration `0127_*` (not applied)
- Canonical evidence: `../cobo_web_design/docs/ai-cache/adhoc-proposal-deadline-day-type-2026-08-10/`
- No DEV deploy / no Production

# Cursor Skill Pack for Cobo Repos

## TÃ­n hiá»‡u tuÃ¢n thá»§ â€” pháº£i tháº¥y Ä‘Æ°á»£c trong Chat (báº¯t buá»™c)

Giá»‘ng `cobo_web_design/docs/ai-cache/README.md`: má»i cÃ¢u tráº£ lá»i cÃ³ ná»™i dung pháº£i cÃ³ **dÃ²ng Ä‘áº§u** báº¯t Ä‘áº§u **`[ai-cache]`** + README + file `docs/ai-cache/` Ä‘Ã£ dÃ¹ng + skill + **`Mandatory README: Ä‘Ã£ Ã¡p dá»¥ng`**. Chi tiáº¿t: xem README trong `cobo_web_design` hoáº·c sao chÃ©p má»¥c Ä‘Ã³ vÃ o repo nÃ y náº¿u lÃ m viá»‡c chá»‰ IAM.

Snippet **â€œBáº¯t buá»™c: tuÃ¢n thá»§ docs/ai-cache/README.mdâ€¦â€** (dÃ¡n Ä‘áº§u prompt) vÃ  **lá»‡nh Docker/build hoáº·c `BLOCKED:`** sau implement: xem **`cobo_web_design/docs/ai-cache/README.md`** vÃ  báº£n siáº¿t trong **`.cursor/rules/ai-cache-read-first.mdc`** cá»§a repo nÃ y.

**Ãp dá»¥ng tá»± Ä‘á»™ng (Cursor):** Luáº­t **`.cursor/rules/ai-cache-read-first.mdc`** (`alwaysApply: true`) + **`AGENTS.md`** á»Ÿ root.

Pack nÃ y gá»“m 2 bá»™ cáº¥u hÃ¬nh:
- `cobo_web_design/.cursor/...`
- `cobo_iam_services/.cursor/...`

## CÃ¡ch dÃ¹ng
1. Copy thÆ° má»¥c `.cursor` trong tá»«ng repo vÃ o Ä‘Ãºng project tÆ°Æ¡ng á»©ng.
2. Giá»¯ cÃ¡c `rules/*.mdc` Ä‘á»ƒ luÃ´n báº­t guardrails kiáº¿n trÃºc.
3. DÃ¹ng Agent trong Cursor vÃ  prompt theo skill tÆ°Æ¡ng á»©ng.
4. Vá»›i task lá»›n, báº¯t Ä‘áº§u báº±ng `system-design-feature`.
5. TrÆ°á»›c khi hoÃ n táº¥t, luÃ´n cháº¡y `premerge-system-review`.

## Prompt máº·c Ä‘á»‹nh nÃªn dÃ¡n cho háº§u háº¿t má»i cÃ¢u há»i

DÃ¹ng prompt nÃ y nhÆ° prompt khá»Ÿi Ä‘áº§u gáº§n nhÆ° má»—i láº§n há»i Cursor.

```text
Use the relevant project skill for this task.
First identify the architectural boundary, affected layers, domain invariants, failure modes, validation strategy, and test scope before writing code.
Preserve backward compatibility unless explicitly asked otherwise.
Prefer minimal, reviewable diffs.
Do not skip loading/error/empty states on frontend.
Do not skip validation, authorization, idempotency, migration safety, or observability on backend.
Before marking the task done, run a pre-merge review and report risks, gaps, and verification steps.
```




## Ad-hoc proposal custom workflow â€” Phase 3.6 multi-step runtime (2026-08-07)

- Verdict **PHASE_3_6_ADHOC_PROPOSAL_MULTI_STEP_RUNTIME_READY**
- Evidence: `docs/ai-cache/adhoc-proposal-custom-workflow-contract-2026-08-07/` (`64`â€“`76`)
- Lazy one-active-task chain; instance completes only after final frozen step; **NO_DEV_DEPLOY**

## Ad-hoc proposal deadline day type â€” plan (2026-08-10)

- Verdict **ADHOC_PROPOSAL_DEADLINE_DAY_TYPE_IMPLEMENTATION_PLAN_READY** (pointer)
- Canonical evidence: `../cobo_web_design/docs/ai-cache/adhoc-proposal-deadline-day-type-2026-08-10/`
- Docs-only FE commit; no BE source this phase

## Ad-hoc proposal custom workflow â€” Phase 3.5 assignment convergence (2026-08-07)

- Verdict **PHASE_3_5_ADHOC_PROPOSAL_ASSIGNMENT_CONTRACT_READY**
- Submit-time `ValidateWorkflowForSubmit` for schema v2; draft incomplete still allowed; evidence under FE `docs/ai-cache/adhoc-proposal-custom-workflow-contract-2026-08-07/` (`54`â€“`63`)
- **NO_DEV_DEPLOY** until Phase 4

## Ad-hoc proposal custom workflow â€” Phase 3 runtime (2026-08-07)

- Verdict **PHASE_3_ADHOC_PROPOSAL_CUSTOM_WORKFLOW_RUNTIME_READY**
- Source in this repo; evidence canonical under sibling FE `docs/ai-cache/adhoc-proposal-custom-workflow-contract-2026-08-07/` (`40`â€“`53`)
- `runtimeV2Implemented=true`; assignment `V2_DIRECT_ASSIGNEE_REQUIRED`; **NO_DEV_DEPLOY** until Phase 4
- Next: Phase 3.6 multi-step chain complete â€” await Phase 4

## Ad-hoc proposal custom workflow â€” Phase 1 (2026-08-07)

- Verdict **PHASE_1_ADHOC_PROPOSAL_CUSTOM_WORKFLOW_BACKEND_FOUNDATION_READY**
- Source in this repo; evidence canonical under sibling FE `docs/ai-cache/adhoc-proposal-custom-workflow-contract-2026-08-07/`
- `NO_MIGRATION_REQUIRED`; Phase 3 runtime landed (see above)
- Next: await Phase 4

## Company department metric â€” Phase 2 DEV (2026-08-06)

- Verdict **PHASE_2_COMPANY_DEPARTMENT_METRIC_DEV_READY** (pointer)
- BE commit deployed: `a9d03fb`
- Canonical evidence: `../cobo_web_design/docs/ai-cache/company-department-metric-2026-08-06/`
- Status: stop at DEV â€” no Production

## Company department metric â€” Phase 1 (2026-08-06)

- Verdict **PHASE_1_COMPANY_DEPARTMENT_METRIC_SOURCE_READY** (pointer)
- Additive `department_count` on company profile / `GetCompanyPlatform`
- Canonical evidence: `../cobo_web_design/docs/ai-cache/company-department-metric-2026-08-06/`
- Next: await Phase 2 DEV deploy confirm

## Company Premium End-to-End â€” Phase 8 final handoff (2026-08-04)

- Verdict **COMPANY_PREMIUM_DEV_READY**: `docs/ai-cache/company-premium-implementation-2026-08-04/` (`36`â€“`46`, `results.json`)
- Lineage + contract + nginx + security + rollback reconciled; Phase 8 docs-only; Production untouched
- FE mirror: `../cobo_web_design/docs/ai-cache/company-premium-fe-dev-2026-08-04/` (`21`â€“`23`)

## Company Premium implementation â€” Phase 5 Backend DEV (2026-08-04)

- Verdict **PHASE_5_BACKEND_DEV_READY**: `docs/ai-cache/company-premium-implementation-2026-08-04/` (`20`â€“`35`)
- DEV migrate 0125 + seed; live concurrency PASS_DEV; API deployed `dd0ff1e`; smoke/authz/FE compat PASS
- Superseded by Phase 8 final verdict above

## Company Premium implementation â€” Phase 4 backend quality (2026-08-04)

- Verdict **PHASE_4_BACKEND_QUALITY_READY**: `docs/ai-cache/company-premium-implementation-2026-08-04/` (`14`â€“`19`)
- PatchOwnCompany plan-before-mutation fix; STRICT/security/migration static gates; task failures 0
- Phase 5 complete â€” see entry above

## Company Premium implementation â€” Phase 3 API exposure (2026-08-04)

- Verdict **PHASE_3_API_EXPOSURE_READY**: `docs/ai-cache/company-premium-implementation-2026-08-04/` (`10`â€“`13`)
- Additive `plan` on GetOwnCompany + `/me/companies`; STRICT enrichment errors; batch reader; no FE/deploy/migrate-DEV
- Open risk carried: `MYSQL_CONCURRENCY_VALIDATION_PENDING_PHASE_5` (not Phase 3 gate)
- Phase 4 complete â€” see entry above

## Company Premium implementation â€” Phase 2 shared Reader (2026-08-04)

- Verdict **PHASE_2_SHARED_READER_READY** (+ `MYSQL_CONCURRENCY_VALIDATION_PENDING_PHASE_5`): `docs/ai-cache/company-premium-implementation-2026-08-04/` (`06`â€“`09`)
- Shared `companyplan.Service`; 0126 retracted â†’ DEV seed; parent-company FOR UPDATE; no API/FE/deploy
- Phase 3 API exposure complete â€” see entry above

## Company Premium implementation â€” Phase 1 domain foundation (2026-08-04)

- Verdict **PHASE_1_DOMAIN_FOUNDATION_READY**: `docs/ai-cache/company-premium-implementation-2026-08-04/` (`03`â€“`05`)
- Package `internal/subscription/companyplan`; schema `0125`; DEV fixtures moved to seed in Phase 2; tests PASS; no API/deploy/migrate-DEV

## Company Premium implementation â€” Phase 0 contract lock (2026-08-04)

- Verdict **PHASE_0_CONTRACT_LOCKED**: `docs/ai-cache/company-premium-implementation-2026-08-04/`
- Case C approved via user instruction 2026-08-04; `company_subscriptions` SoT; `plan: null` no-plan; badge = PREMIUM+ACTIVE+COMPANY_SUBSCRIPTION
- Awaiting user confirmation before Phase 1 (domain + migration)

## Company Premium Backend implementation plan (2026-08-03)

- Verdict **COMPANY_PREMIUM_IMPLEMENTATION_PLAN_READY**: `docs/ai-cache/company-premium-backend-implementation-plan-2026-08-03/`
- Deep audit @ `9284a31`; Case B paid-plan source; A+B API `RECOMMENDED_PENDING_APPROVAL`; exact file map + phases; no source/migration/deploy

## Operational Dashboard real KPI aggregation (2026-08-03)

- Verdict **DASHBOARD_REAL_KPI_IMPLEMENTATION_READY** â€” pointer `docs/ai-cache/dashboard-kpi-real-aggregation-2026-08-03/`; canonical pack in FE sibling
- DEV deploy smoke **PASS_WITH_DATA_LIMITATIONS** â€” see `20-dev-deploy-pointer.md` + FE `results.json`
- Remap overview KPIs + completed_at read-only; no migration/deploy

## Workflow alert tasks 500 + source UX (DEV)

- Verdict **PASS_TASKS_API_AND_WORKFLOW_SOURCE_UX** (2026-07-30): `docs/ai-cache/workflow-alert-tasks-500-and-source-ux-remediation-2026-07-30/`
- T500-RC4 NULL department Scan; CMS dual-source Global Template v1 Â· 4 steps; no rematerialize

## Resolved deadline rule by company (Phase 5 DEV deploy)

- Verdict **PHASE_5_DEV_DEPLOYMENT_READY** (2026-07-31): pointer `00-phase-5-pointer.md`; full evidence in FE pack `cobo_web_design/.../resolved-deadline-rule-implementation-2026-07-31/`
- API-only additive deploy from `e2e3f1c`; worker/MySQL unchanged; no migration; await Phase 6

## Resolved deadline rule by company (Phase 2 backend)

- Verdict **PHASE_2_BACKEND_READY** (2026-07-31): `docs/ai-cache/resolved-deadline-rule-implementation-2026-07-31/`
- Additive `resolved_deadline_rule` on type detail; reuses ResolveStructure / ResolveDeadlineDays; no deploy

## Workflow config vs deadline alert steps (DEV audit)

- Verdict **EXPECTED_RUNTIME_REPRESENTATION** (2026-07-30): `docs/ai-cache/workflow-config-vs-deadline-alert-steps-audit-2026-07-30/`
- RC-4 dual-SoT; snapshot/alert steps match effective; CMS global config empty
- Paired FE screenshots under same folder name in `cobo_web_design`

## Guarded periodic one-shot materialization (DEV)

- Verdict **PASS_ONE_SHOT_MATERIALIZATION_AND_ALERT_E2E** (2026-07-30): `docs/ai-cache/guarded-periodic-one-shot-materialization-2026-07-30/`
- CLI one-shot exact QA scope; production calculator/materializer; seeding remains OFF
- Paired FE UI evidence under same folder name in `cobo_web_design`

## Prompt tÃ¡i sá»­ dá»¥ng theo tá»«ng tÃ¬nh huá»‘ng

### 1) Khi xÃ¢y feature má»›i tá»« Ä‘áº§u

```text
Use system-design-feature first, then switch to the relevant repo-specific implementation skill.
Before coding, define the objective, user flow, domain invariants, API contract, UI states, data flow, failure modes, rollout approach, and test plan.
Only then implement with minimal and reviewable diffs.
```

### 2) Khi lÃ m feature frontend trong `cobo_web_design`

```text
Use the frontend skill that best matches this task.
Follow the vertical slice structure: route -> screen -> feature components -> hooks/services -> types.
Do not mix route concerns, fetching concerns, and presentation concerns in one large file.
Handle loading, error, empty, success, disabled, and invalid-param states explicitly.
Add focused Vitest/testing-library coverage for the core user-visible behavior.
```

### 3) Khi lÃ m feature backend trong `cobo_iam_services`

```text
Use the backend skill that best matches this task.
Keep boundaries clear: handler -> service/usecase -> repository -> external systems.
Define request/response contract, validation rules, authorization rules, transaction boundaries, cache impact, retry/idempotency considerations, and test matrix before coding.
Do not hide security, migration, or data consistency risks.
```

### 4) Khi sá»­a bug

```text
Use the relevant debugging or repo-specific skill.
First restate the symptom, expected behavior, actual behavior, likely root causes, and the most probable failure path from code.
Fix the root cause with the smallest safe change, then add regression protection and list any remaining uncertainty.
```

### 5) Khi review trÆ°á»›c merge

```text
Run premerge-system-review.
Audit requirement coverage, architectural fit, frontend state completeness, validation completeness, API/contract consistency, auth/security risks, data consistency risks, migration/deployment risks, observability gaps, and missing regression tests.
Group findings into critical, important, and nice-to-have.
```

## Prompt siÃªu ngáº¯n Ä‘á»ƒ ghim cá»‘ Ä‘á»‹nh

Náº¿u báº¡n muá»‘n má»™t prompt ngáº¯n hÆ¡n Ä‘á»ƒ dÃ¹ng liÃªn tá»¥c:

```text
Use the relevant project skill. Think in layers, define contracts first, handle failure modes explicitly, keep changes minimal, and do a system-level review before done.
```

## Máº¹o dÃ¹ng thá»±c táº¿
- Vá»›i task mÆ¡ há»“: luÃ´n báº¯t Ä‘áº§u báº±ng prompt feature má»›i.
- Vá»›i task chá»‰ cháº¡m UI: dÃ¹ng prompt frontend.
- Vá»›i task auth/API/data: dÃ¹ng prompt backend.
- Vá»›i bug khÃ³: dÃ¹ng prompt sá»­a bug.
- Vá»›i PR sáº¯p xong: dÃ¹ng prompt review trÆ°á»›c merge.

## Prompt khuyÃªn dÃ¹ng cÅ©

```text
Use the relevant project skill for this task. Start by identifying the architectural boundary, domain invariants, failure modes, and validation strategy before coding. Prefer minimal, reviewable diffs and preserve backward compatibility unless explicitly asked otherwise.
```

```text
For this feature, use system-design-feature first, then use the repo-specific implementation skill. Do not start coding until you have listed API contract, UI states, data flow, edge cases, and test plan.
```

```text
Before marking this done, run premerge-system-review and report missing validation, missing UI states, contract mismatches, auth risks, data consistency risks, and regression gaps.
```

## Mandatory Prompt Requirement (2 repos)

Ãp dá»¥ng báº¯t buá»™c cho má»i prompt liÃªn quan `cobo_web_design` vÃ /hoáº·c `cobo_iam_services`:

1. TrÆ°á»›c má»i bÆ°á»›c, Ä‘á»c `docs/ai-cache/README.md` vÃ  toÃ n bá»™ context tÃ¡i sá»­ dá»¥ng trong `docs/ai-cache/`.
2. Thá»© tá»± Æ°u tiÃªn khi cÃ³ xung Ä‘á»™t:
   - `docs/ai-cache/README.md`
   - cÃ¡c file cÃ²n láº¡i trong `docs/ai-cache/`
   - project rules
   - docs/pattern cÅ© trong repo
3. Chá»n skill phÃ¹ há»£p; náº¿u task cháº¡m cáº£ 2 repo thÃ¬ dÃ¹ng `integration-cross-repo`.
4. Vá»›i feature má»›i: báº¯t buá»™c contract-first trÆ°á»›c khi code (contract + request/response/error matrix + FE mapping + BE expectations + failure modes + rollout risks + validation plan).
5. Vá»›i task implement:
   - diff nhá», dá»… review
   - káº¿t thÃºc báº±ng `premerge-system-review`
   - sau má»—i cycle cÃ³ thay code pháº£i rerun fresh Docker build cho services bá»‹ áº£nh hÆ°á»Ÿng
   - khÃ´ng coi lÃ  xong cho tá»›i khi Docker build má»›i nháº¥t Ä‘Ã£ cháº¡y vÃ  bÃ¡o káº¿t quáº£
6. Vá»›i task phÃ¢n tÃ­ch/review:
   - khÃ´ng sá»­a code náº¿u chÆ°a Ä‘Æ°á»£c yÃªu cáº§u explicit
   - phÃ¢n tÃ­ch dá»±a trÃªn code/docs thá»±c táº¿, khÃ´ng phá»ng Ä‘oÃ¡n
7. Sau má»—i task (implement hoáº·c understand), ghi tÃ³m táº¯t tÃ¡i sá»­ dá»¥ng vÃ o `docs/ai-cache/` theo format ngáº¯n, nháº¥t quÃ¡n:
   - task type
   - objective/question
   - implemented/discovered
   - affected repos/files/modules
   - contracts/behaviors/constraints/decisions
   - build/verification result (náº¿u cÃ³)
   - remaining gaps/risks/next steps
## Workflow Configuration load 404 (DEV)

- Canonical audit (FE pack): `../cobo_web_design/docs/ai-cache/workflow-config-load-404-audit-2026-07-30/`
- Remediation (FE pack): `../cobo_web_design/docs/ai-cache/workflow-config-load-404-remediation-2026-07-30/` â€” DEV flag ON, isolation PASS
- **ROOT_CAUSE_CONFIRMED** RC-4 then remediated: `WORKFLOW_VERSIONING_ENABLED` was OFF â†’ routes not registered
- Mirror note: `reusable-task-updates.md` (2026-07-30)

## Current-month periodic deadline alert E2E (DEV)

- Canonical FE pack: `../cobo_web_design/docs/ai-cache/current-month-periodic-deadline-alert-e2e-2026-07-30/`
- Verdict: **BLOCKED_PERIODIC_MATERIALIZATION** â€” QA active; seeding remains OFF
- Materialization later unblocked by guarded one-shot pack above

## Q2 Financial Report Deadline Alert E2E (DEV)

- Canonical (FE pack): `../cobo_web_design/docs/ai-cache/q2-financial-report-deadline-alert-e2e-2026-07-30/`
- Verdict: **BLOCKED_DEADLINE_CONTRACT** â€” no PERIOD_END in deadline engine/CMS; seeding remains OFF

## Deadline alert â€” active report missing? (DEV)

- Canonical audit (FE pack): `../cobo_web_design/docs/ai-cache/deadline-alert-active-report-audit-2026-07-30/`
- Verdict: **EXPECTED_BEHAVIOR_CONFIRMED** (RC-1) â€” `bao-cao-tai-chinh-quy-2` on `c_001` returns `PENDING_CONFIRM`, not open OVERDUE
- Flags on DEV: `PERIODIC_SEEDING_ENABLED=false`, `DEADLINE_ENGINE_V2=false` (shadow true)

## Legal Basis Phase 12.6 (IAM mirror)

- Phase 12.6A: operational `PASS_READ_ONLY_DRY_RUN`; governance `FAIL_SCOPE_CREEP` â€” see plan folder `phase-12-6a-scope-exception.md`
- Phase 12.6B-Plan: `BACKFILL_PLAN_READY` â€” `phase-12-6b-plan-handoff.md` (docs only; no apply)

