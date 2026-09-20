## Tenant alert complete ≠ publish — residual close-out PASS (2026-09-20)

- Pack (FE sibling): `tenant-alert-complete-action-plan-2026-09-20/` (`04`–`06`)
- DEV complete-click + draft smoke PASS; no external publication
- READY_FOR_USER_COMMIT=true; NO agent commit/push/prod

## Tenant alerts unified - blocker fix + DEV smoke PASS (2026-09-20)

- Pack (FE sibling): `tenant-alerts-unified-proposals-plan-2026-09-19/03` + `04`
- BE: GetAlertRowByRecordID detail authz; catalog read allows ad_hoc_alert.propose
- DEV_BROWSER_SMOKE=PASS; READY_FOR_USER_COMMIT=true; NO agent commit/push/prod

## Tenant alerts unified ? DEV edge-case smoke PARTIAL (2026-09-20)

- Pack (FE sibling): `tenant-alerts-unified-proposals-plan-2026-09-19/03` + `04-dev-edge-case-smoke-result.md`
- Approved link PARTIAL; propose-only route FAIL; 500 isolation PASS; proposal 403 FAIL; empty PASS
- BE unchanged; NO agent commit/push/prod
## Tenant alerts + ad-hoc proposals unified ? DEV smoke PARTIAL (2026-09-20)

- Pack: sibling FE `tenant-alerts-unified-proposals-plan-2026-09-19/03-dev-smoke-result.md`
- BE unchanged; FE deploy only; DEV_BROWSER_SMOKE=PARTIAL
- READY_FOR_USER_COMMIT=true; NO BE code/commit
## Tenant alerts + ad-hoc proposals unified screen ? Phase 1 FE (2026-09-20)

- Pack: `tenant-alerts-unified-proposals-plan-2026-09-19/` (full solution in FE sibling)
- Phase 1 BE unchanged; `BE_UNIFIED_FEED=DEFERRED`; FE parallel existing APIs
- Regression PASS; READY_FOR_USER_COMMIT=true (FE); NO BE code/commit
## Tenant alerts + ad-hoc proposals unified screen - PLAN (2026-09-19)

- Pack: `tenant-alerts-unified-proposals-plan-2026-09-19/` (full solution in FE sibling)
- Phase 1 BE unchanged; optional Phase 2 alerts-feed; READY_FOR_IMPLEMENTATION=true; NO code
## Tenant Deadline Detail action UX - DEV browser smoke PASS (2026-09-19)

- Pack: `tenant-step-sequential-unlock-plan-2026-09-19/10-action-ux-dev-smoke-result.md` (full: sibling FE)
- FE deploy only; asset `index-DQsG-Kl9.js`; BE unchanged; READY_FOR_USER_COMMIT=true; NO agent commit
## Tenant Deadline Detail action UX (2026-09-19)

- Pack: `tenant-deadline-detail-action-ux-2026-09-19/` (full: sibling FE)
- FE-only; BE unchanged; READY_FOR_USER_COMMIT=true
## Tenant step sequential unlock - DEV smoke PASS (2026-09-19)

- Pack: `tenant-step-sequential-unlock-plan-2026-09-19/` (+ `09-dev-smoke-result.md`)
- DEV be+fe via `deploy-dev.ps1`; Step2 early unlock + rename button verified API+browser
- READY_FOR_USER_COMMIT=true; NO agent commit/push/prod
## Tenant step sequential unlock - implemented (2026-09-19)

- Pack: `tenant-step-sequential-unlock-plan-2026-09-19/` (00, 06, 07, 08)
- Product: EARLY_START_FIRST_STEP=false; SUCCESSOR_UNLOCK_AFTER_PREDECESSOR_COMPLETION=true; rename alert button
- CODE_MODIFIED=true; MIGRATION=false; DEV smoke NOT run; READY_FOR_USER_COMMIT=true
## Tenant step discussion comments - implementation plan (2026-09-19)

- Pointer: `tenant-step-discussion-comments-implementation-plan-2026-09-19/00-pointer.md`
- V1: COMPLETE_DEV_VERIFIED; READY_FOR_USER_COMMIT=true; NO agent commit
- V1.1 Phase 1-6 DEV VERIFIED; 0140 applied; READY_FOR_USER_COMMIT=true (`06-v1-1-dev-verification-result.md`)

## Tenant generic step evidence V1.1 - G5 Final Pre-Commit Review (2026-09-18)

- Pointer: `tenant-generic-step-evidence-v1-1-g5-final-precommit-review-2026-09-18/`
- Full pack: sibling cobo_web_design; READY_FOR_USER_COMMIT=true - WAIT_FOR_USER_COMMIT

## Tenant generic step evidence V1.1 - G4 Full Integrated E2E + Release Verification (2026-09-18)

- Pointer: `tenant-generic-step-evidence-v1-1-g4-full-release-verification-2026-09-18/00-pointer.md`
- Full pack: sibling cobo_web_design; verification-only; READY_FOR_G5=true - WAIT_FOR_PO_CONFIRMATION

## Tenant generic step evidence V1.1 - G3 Tenant UX (2026-09-18)

- Pointer: sibling FE `../cobo_web_design/docs/ai-cache/tenant-generic-step-evidence-v1-1-g3-tenant-ux-2026-09-18/`
- BE product unchanged this phase; READY_FOR_G4=true - WAIT_FOR_PO_CONFIRMATION

## Tenant generic step evidence V1.1 - G2C audit/security/concurrency hardening (2026-09-18)

- Pack: `tenant-generic-step-evidence-v1-1-g2c-audit-security-hardening-2026-09-18/`
- AFTER_COMMIT_BEST_EFFORT audit upload/delete/replace; WORKFLOW_STEP_EVIDENCE_* error domain; DEV audit+retention smoke PASS
- G2C_PHASE_RESULT=PASS; READY_FOR_G3=true - WAIT_FOR_PO_CONFIRMATION (NO_FE/NO_COMMIT)

## Tenant generic step evidence V1.1 - G2B CRUD API (2026-09-18)

- Pack: `tenant-generic-step-evidence-v1-1-g2b-crud-api-2026-09-18/`
- 5 additive `/evidence-files` routes; BE capabilities; DEV smoke zero-req upload PASS
- G2B_PHASE_RESULT=PASS; READY_FOR_G2C=true - WAIT_FOR_PO_CONFIRMATION (NO_FE/NO_AUDIT/NO_COMMIT)

## Tenant generic step evidence V1.1 - G2A runtime domain (2026-09-18)

- Pack: `tenant-generic-step-evidence-v1-1-g2a-runtime-domain-2026-09-18/`
- Migration 0137 `workflow_step_evidence_files`; package `internal/workflowstepevidence`
- G2A_PHASE_RESULT=PASS; READY_FOR_G2B=true - WAIT_FOR_PO_CONFIRMATION (NO_FE/NO_API/NO_COMMIT)

## Tenant generic step evidence V1.1 - G1 contract + source audit (2026-09-18)

- Pack: `tenant-generic-step-evidence-v1-1-g1-contract-source-audit-2026-09-18/`
- TABLE_STRATEGY=DEDICATED_GENERIC_EVIDENCE_TABLE (`workflow_step_evidence_files`)
- G1_PHASE_RESULT=PASS; READY_FOR_G2=true - WAIT_FOR_PO_CONFIRMATION (NO_IMPLEMENTATION)

## Tenant workflow step evidence V1 - final task completion (2026-09-18)

- Pack: `tenant-workflow-step-evidence-v1-final-task-completion-2026-09-18/`
- EVIDENCE_V1_FINAL_TASK_COMPLETION_REVIEW=PASS; TASK_COMPLETED=true; READY_FOR_USER_COMMIT=true
- AGENT_COMMIT_ALLOWED=false; COMMIT_PERFORMED=false - WAIT_FOR_USER_COMMIT

## Tenant workflow step evidence V1 - pre-commit final review (2026-09-18)

- Pack: `tenant-workflow-step-evidence-v1-precommit-final-review-2026-09-18/`
- PRE_COMMIT_FINAL_REVIEW=PASS; READY_FOR_COMMIT=true - WAIT_FOR_PO_CONFIRMATION (NO_COMMIT)

## Tenant evidence authoring recovery - real materialization E2E (2026-09-18)

- Pack: `tenant-evidence-authoring-data-correction-real-e2e-2026-09-18/`
- ROOT_CAUSE=AUTHORING_EMPTY_DOCUMENTS_ZERO_SNAPSHOT corrected via QA authoring + real B1 path
- EVIDENCE_V1_DEV_DONE restored; READY_FOR_COMMIT=true - WAIT_FOR_PO_CONFIRMATION

## Tenant evidence upload runtime defect diagnosis (2026-09-18)

- Pack: `tenant-evidence-upload-runtime-defect-diagnosis-2026-09-18/00-diagnosis.md`
- Same as FE pack; SOURCE_CHANGED=false

## Tenant workflow step evidence B6 - Full V1 release verification (2026-09-14)

- Pointer: `tenant-workflow-step-evidence-b6-full-v1-release-verification-2026-09-14/00-pointer.md`
- Full: sibling FE pack
- Verification-only; EVIDENCE_V1_DEV_DONE=true - WAIT_FOR_PO_CONFIRMATION

## Tenant workflow step evidence B5 - Audit / lifecycle / security hardening (2026-09-14)

- Pointer: `tenant-workflow-step-evidence-b5-audit-security-hardening-2026-09-14/00-pointer.md`
- Full: sibling FE `../cobo_web_design/docs/ai-cache/tenant-workflow-step-evidence-b5-audit-security-hardening-2026-09-14/`
- BE only: audit upload/delete/replace + lifecycle + security/concurrency; no FE/DB
- BE_SOURCE_CHANGED=true; READY_FOR_B6=true - WAIT_FOR_PO_CONFIRMATION

## Tenant workflow step evidence B4 - Tenant runtime fulfillment UX (2026-09-14)

- Pointer: `tenant-workflow-step-evidence-b4-tenant-ux-2026-09-14/00-pointer.md`
- Full: sibling FE `../cobo_web_design/docs/ai-cache/tenant-workflow-step-evidence-b4-tenant-ux-2026-09-14/`
- FE-only B?ng ch?ng UX on B2/B3; BE unchanged
- FE_SOURCE_CHANGED=true; READY_FOR_B5=true - WAIT_FOR_PO_CONFIRMATION

## Tenant workflow step evidence B3 - Complete required-document gate (2026-09-13)

- Pointer: `tenant-workflow-step-evidence-b3-complete-gate-2026-09-13/00-pointer.md`
- Full: sibling FE `../cobo_web_design/docs/ai-cache/tenant-workflow-step-evidence-b3-complete-gate-2026-09-13/`
- UpsertStepCompleted TX gate on B1 snapshots + B2 ACTIVE; error WORKFLOW_STEP_REQUIRED_DOCUMENT_MISSING (422); legacy zero-snap preserved
- BE_SOURCE_CHANGED=true; FE/DB/API file surface unchanged; READY_FOR_B4=true - WAIT_FOR_PO_CONFIRMATION

## Tenant workflow step evidence B2 - runtime fulfillment (2026-09-13)

- Pointer: `tenant-workflow-step-evidence-b2-runtime-fulfillment-2026-09-13/00-pointer.md`
- Full: sibling FE `../cobo_web_design/docs/ai-cache/tenant-workflow-step-evidence-b2-runtime-fulfillment-2026-09-13/`
- Migration 0136 + fulfillment CRUD/ACL/replace lineage; DEV deploy+smoke PASS
- BE_SOURCE_CHANGED=true; READY_FOR_B3=true - WAIT_FOR_PO_CONFIRMATION

## Tenant workflow step evidence B1 - requirement snapshot (2026-09-13)

- Pointer: `tenant-workflow-step-evidence-b1-requirement-snapshot-2026-09-13/00-pointer.md`
- Full: sibling FE `../cobo_web_design/docs/ai-cache/tenant-workflow-step-evidence-b1-requirement-snapshot-2026-09-13/`
- Migration 0135 + CreateWorkflowInstanceInternal TX snapshot; DEV deploy+smoke PASS
- BE_SOURCE_CHANGED=true; READY_FOR_B2=true - WAIT_FOR_PO_CONFIRMATION

## Tenant workflow step evidence V1 - contract + implementation plan (2026-09-13)

- Pointer: `tenant-workflow-step-evidence-v1-contract-plan-2026-09-13/00-pointer.md`
- Full: sibling FE `../cobo_web_design/docs/ai-cache/tenant-workflow-step-evidence-v1-contract-plan-2026-09-13/`
- Document Requirement Fulfillment; snapshot at CreateWorkflowInstanceInternal; Complete gate; B1-B6 roadmap
- BE_SOURCE_CHANGED=false; READY_FOR_B1=true - WAIT_FOR_PO_CONFIRMATION

## Tenant workflow step evidence / runtime upload audit (2026-09-13)

- Pointer: `tenant-workflow-step-evidence-runtime-upload-audit-2026-09-13/00-pointer.md`
- Full evidence in sibling FE `../cobo_web_design/docs/ai-cache/tenant-workflow-step-evidence-runtime-upload-audit-2026-09-13/`
- DEEP_SOURCE_AUDIT_ONLY: Step Detail "B?ng ch?ng"; no runtime fulfillment; media reusable; feature LARGE
- BE_SOURCE_CHANGED=false; READY_FOR_IMPLEMENTATION_PLAN=true - WAIT_FOR_PO_CONFIRMATION

## Tenant Step Detail - remove T-c v? tab (2026-09-13)

- Pointer: `tenant-step-detail-task-section-removal-2026-09-13/00-pointer.md`
- Full evidence in sibling FE `../cobo_web_design/docs/ai-cache/tenant-step-detail-task-section-removal-2026-09-13/`
- FE-only presentation removal; WorkflowCard/listTasks/actions preserved; BE unchanged
- BE_SOURCE_CHANGED=false; FE deploy only; READY_FOR_COMMIT=true - WAIT_FOR_PO_CONFIRMATION

## Tenant Step Detail - task section removal delta check (2026-09-13)

- Pointer: `tenant-step-detail-task-section-removal-delta-check-2026-09-13/00-pointer.md`
- Full evidence in sibling FE `../cobo_web_design/docs/ai-cache/tenant-step-detail-task-section-removal-delta-check-2026-09-13/`
- SOURCE_CHECK_ONLY: no unique UI info in Step Detail T-c v? tab; recommend REMOVE_PRESENTATION
- BE_SOURCE_CHANGED=false; NO_IMPLEMENTATION - WAIT_FOR_PO_CONFIRMATION

## Tenant Step Detail - timeliness + task presentation fix (2026-09-13)

- Pointer: `tenant-step-detail-timeliness-task-presentation-fix-2026-09-13/00-pointer.md`
- Full evidence in sibling FE `../cobo_web_design/docs/ai-cache/tenant-step-detail-timeliness-task-presentation-fix-2026-09-13/`
- FE-only: consume BE `timeliness_status`; relabel T-c v?; BE unchanged
- BE_SOURCE_CHANGED=false; FE deploy only; READY_FOR_COMMIT=true - WAIT_FOR_PO_CONFIRMATION

## Tenant Step Detail - timeliness + "T-c v? b?t bu?c" audit (2026-09-13)

- Pointer: `tenant-step-detail-timeliness-required-task-audit-2026-09-13/00-pointer.md`
- Full evidence in sibling FE `../cobo_web_design/docs/ai-cache/tenant-step-detail-timeliness-required-task-audit-2026-09-13/`
- SOURCE_AUDIT_ONLY: Step Detail remaining metric ignores BE `timeliness_status`; T-c v? b?t bu?c is FE label over TaskDTO
- BE_SOURCE_CHANGED=false; NO_IMPLEMENTATION - WAIT_FOR_PO_CONFIRMATION

## Tenant step description readiness v1 (2026-09-13)

- Pointer: `tenant-step-description-activation-readiness-v1-2026-09-13/00-pointer.md`
- Full evidence in sibling FE `../cobo_web_design/docs/ai-cache/tenant-step-description-activation-readiness-v1-2026-09-13/`
- BE shared semantic description gate on Template Activate + override approve/apply
- BE_SOURCE_CHANGED=true; DEV deploy PASS; READY_FOR_COMMIT=true - WAIT_FOR_PO_CONFIRMATION

## Tenant step description - activation readiness audit (2026-09-13)

- Pointer: `tenant-step-description-activation-readiness-audit-2026-09-13/00-pointer.md`
- Full evidence in sibling FE `../cobo_web_design/docs/ai-cache/tenant-step-description-activation-readiness-audit-2026-09-13/`
- PLAN_ONLY: description required on Activate?; BE readiness authority confirmed; no description blocker today
- BE_SOURCE_CHANGED=false; NO_IMPLEMENTATION - WAIT_FOR_PO_CONFIRMATION

## Workflow authoring data correction plan (2026-09-13)

- Pointer: sibling FE `docs/ai-cache/workflow-authoring-data-correction-plan-2026-09-13/`
- PLAN_ONLY; no CMS/DB mutation

## Tenant workflow authoring data audit (2026-09-13)

- Pointer: sibling FE `docs/ai-cache/tenant-workflow-authoring-data-audit-2026-09-13/`
- SOURCE_DATA_AUDIT_ONLY; APPLICATION_SOURCE_CHANGED=false; no DB mutation

## Tenant step detail content + role display fix (2026-09-13)

- Pointer: FE-only; BE unchanged. Evidence in sibling `cobo_web_design/docs/ai-cache/tenant-step-detail-content-role-display-fix-2026-09-13/`
- BE_SOURCE_CHANGED=false; FE deploy only

## Tenant step detail content + owner authority audit (2026-09-13)

- Pointer: `tenant-step-detail-content-owner-authority-audit-2026-09-13/00-pointer.md`
- Full evidence in sibling FE repo
- SOURCE_AUDIT_ONLY; BE source unchanged

## Tenant current processing department presentation fix (2026-09-13)

- FE-only pointer: sibling `../cobo_web_design/docs/ai-cache/tenant-current-processing-department-presentation-fix-2026-09-13/`
- BE source unchanged; FE `deploy-dev.ps1 -Mode fe` PASS

## Tenant current processing department authority audit (2026-09-13)

- Pointer: `tenant-current-processing-department-authority-audit-2026-09-13/00-pointer.md`
- Full evidence in sibling FE `../cobo_web_design/docs/ai-cache/tenant-current-processing-department-authority-audit-2026-09-13/`
- SOURCE_AUDIT_ONLY; BE source unchanged

## Portal Company resolved deadline display (2026-09-04)

- List API adds `resolved_due_at` (company-scoped); pointer: `portal-company-resolved-deadline-display-2026-09-04/00-pointer.md`
- Full evidence in sibling FE repo `docs/ai-cache/portal-company-resolved-deadline-display-2026-09-04/`
- NO_COMMIT / NO_PUSH / NO_MERGE / NO_PRODUCTION - WAIT_FOR_CONFIRMATION

## Periodic seeding controlled DEV enablement (2026-08-25)

- Enabled worker bundle: `PERIODIC_SEEDING_ENABLED=true` + `WORKFLOW_SNAPSHOT_ENABLED=true` (worker-only recreate)
- First tick: +52 cycles/+52 Draft/+52 workflows; orphans=0; target DAILY 4- UPCOMING; steady-state idempotent
- Evidence: `periodic-seeding-controlled-dev-enablement-2026-08-25/`
- NO_COMMIT / NO_PUSH / NO_MERGE / NO_PRODUCTION - WAIT_FOR_CONFIRMATION

## Periodic seeding DEV enablement impact audit (2026-08-25)

- READ-ONLY: blast radius before `PERIODIC_SEEDING_ENABLED=true`
- Verdict: **CONDITIONAL_GO** (must also set worker `WORKFLOW_SNAPSHOT_ENABLED=true`; ~52 cycles / =52 records / ~28 alerts / ~24 OVERDUE)
- Evidence: `periodic-seeding-dev-enablement-impact-audit-2026-08-25/` (`00`-`15` + matrix)
- NO_CONFIG_CHANGE / NO_WORKER_RESTART / NO_DB_MUTATION - WAIT_FOR_CONFIRMATION

## Deadline Alert - DAILY template root-cause audit (2026-08-25)

- READ-ONLY: template "B?ng t-nh luong nh-n vi-n ng-y" Active DAILY AF frozen but no Portal deadline alert
- Root: DEV `PERIODIC_SEEDING_ENABLED=false` ? no Seed/Materialize ? cycles=0 records=0
- Class `I_PERIODIC_SEEDING_DISABLED`; FIX_LAYER=CONFIG; IMPLEMENTATION_STARTED=false
- Evidence: `deadline-alert-daily-template-root-cause-2026-08-25/` (`00`-`09`)
- NO_FIX / NO_DB_MUTATION / NO_WORKER_TRIGGER / NO_DEPLOY / NO_COMMIT - WAIT_FOR_CONFIRMATION

## Deadline Alert V1 Phase 4 - premerge system review (2026-08-25)

- Review-only: contract/SQL/auth/perf/tests; clean commit candidate manifest; no app source change this phase
- Evidence: `deadline-alert-v1-phase-4-premerge-system-review-2026-08-25/` (`00`-`14`)
- READY_FOR_COMMIT=true (clean files); READY_FOR_PUSH/MERGE/PROD=false; OPEN_P0=0; PERFORMANCE=P1_FOLLOW_UP
- NO_COMMIT / NO_PUSH / NO_MERGE / NO_PRODUCTION - WAIT_FOR_CONFIRMATION

## Deadline Alert V1 Phase 3 - DEV verification (2026-08-25)

- BE-only deploy to avi-server1; healthz/readyz 200; QA API+browser E2E PASS (Draft actionable + Submit removes alert)
- Evidence: `deadline-alert-v1-phase-3-dev-verification-2026-08-25/` (`00`-`12` + screenshots)
- APPLICATION_SOURCE_CHANGED_BY_PHASE_3=false; READY_FOR_PHASE_4_PREMERGE; NO_COMMIT/NO_PUSH/NO_MERGE - WAIT_FOR_CONFIRMATION

## Deadline Alert V1 Phase 2 - service integration (2026-08-25)

- Removed Go `isDraftRecordStatus` membership skip in `ListDeadlineAlerts`; due/status/confirmation preserved
- Evidence: `deadline-alert-v1-phase-2-service-integration-2026-08-25/` (`00`-`07`)
- `go test ./internal/deadlinealerts/...` + `go build ./cmd/api` PASS; Phase 1 mysql regression PASS
- READY_FOR_PHASE_3_DEV_VERIFICATION; NO_DEV_DEPLOY / NO_E2E / NO_COMMIT / NO_PUSH / NO_MERGE - WAIT_FOR_CONFIRMATION

## Deadline Alert V1 Phase 1 - SQL membership (2026-08-25)

- `ListRows` V1 membership: Draft + submitted_at IS NULL; periodic COALESCE(open_at,cycle_start)<=TodayHCM; irregular NOT EXISTS
- Files: `internal/deadlinealerts/infra/mysql/{repository.go,list_rows_membership.go,list_rows_membership_test.go}`
- Evidence: `deadline-alert-v1-phase-1-sql-membership-2026-08-25/` (`00`-`06`)
- Repository tests PASS; GO_DRAFT_FILTER still present; FULL_FEATURE=false; READY_FOR_PHASE_2
- NO_DEV_DEPLOY / NO_E2E / NO_COMMIT / NO_PUSH / NO_MERGE - WAIT_FOR_CONFIRMATION

## Deadline Alert V1 solution plan 2026-08-25

- Pointer: FE `../cobo_web_design/docs/ai-cache/deadline-alert-v1-solution-plan-2026-08-25/`
- PLAN ONLY; APPLICATION_SOURCE_CHANGED=false

## Deadline alert domain review plan 2026-08-25

- Pointer: FE `../cobo_web_design/docs/ai-cache/deadline-alert-domain-review-plan-2026-08-25/`
- PLAN ONLY; APPLICATION_SOURCE_CHANGED=false

## Deadline alert source audit 2026-08-25

- Pointer: FE `../cobo_web_design/docs/ai-cache/deadline-alert-source-audit-2026-08-25/` (+ local `deadline-alert-source-audit-2026-08-25/00-pointer.md`)
- AUDIT ONLY; CODE_CHANGED=false

## Periodicity V2 Phase 0+1 (2026-08-24)

- Foundation + canonical resolver: `cycle_anchor_weekday`, `month_in_quarter`; migration 0133; ResolveOccurrenceT weekly/quarterly.
- Canonical docs: `../cobo_web_design/docs/ai-cache/periodicity-v2-phase-0-1-2026-08-24/`
- DEV migrate+be PASS; READY_FOR_PHASE_2; NO_COMMIT/NO_PUSH/NO_MERGE - WAIT_FOR_CONFIRMATION

## Periodicity V2 impact plan (2026-08-24)

- PLAN ONLY - canonical docs live in sibling FE repo:
- `../cobo_web_design/docs/ai-cache/periodicity-v2-impact-plan-2026-08-24/`
- CODE_CHANGED=false; WAIT_FOR_PO_APPROVAL; NO_COMMIT/NO_PUSH/NO_MERGE

## Cycle anchor day write validation 1..31 (2026-08-24)

- Server hardening: `ValidateCycleAnchorDay` on CMS config/upsert + Company prefs; reject =0 (except unset 0) and =32.
- Evidence: `cycle-anchor-day-write-validation-2026-08-24/`
- ClampDayOfMonth unchanged; no migration; DEV BE deploy + API smoke PASS
- NO_PRODUCTION / NO_COMMIT / NO_PUSH / NO_MERGE - WAIT_FOR_CONFIRMATION

## Workflow Step Document Requirements Full V1 (2026-08-22)

- B1 upload purpose-scoped + 0132 assets table; B2 documents[] + binder; FE editors/preview/portal.
- E2E bugfix: CMS facade documents persist; workflowconfig manifest documents; DEV redeploy.
- Pre-merge gap closed: CMS upsert calls `NormalizeAndValidateWorkflowDocuments` + `validateWorkflowDocumentTemplateRefs(..., "cms")` - evidence `11-cms-server-validation-hardening.md`
- Pointer: FE `../cobo_web_design/docs/ai-cache/workflow-step-document-requirements-2026-08-22/`
- NO_PRODUCTION / NO_COMMIT / NO_PUSH / NO_MERGE

## Workflow Step Document Requirements V1 - STOP (2026-08-22)

- Reconciliation: `documents[{doc_id,name,required}]` already config domain; CMS media PARTIAL (no Company upload / no xlsx ACL).
- Pointer: FE `../cobo_web_design/docs/ai-cache/workflow-step-document-requirements-2026-08-22/`
- Verdict: `IMPLEMENTATION_SAFE_TO_START=false` / WAIT_FOR_PO
- NO_CODE / NO_DB / NO_DEPLOY / NO_COMMIT / NO_PUSH / NO_MERGE

## Company Workflow Step Safe HTML (2026-08-22)

- Company override draft/active validates/normalizes additive `description_format` (same enums as CMS); no migration.
- Pointer: FE `../cobo_web_design/docs/ai-cache/company-workflow-step-safe-html-2026-08-22/`
- Verdict: `COMPANY_WORKFLOW_STEP_SAFE_HTML_DEV_VERIFIED` / Live Activate `NOT_VERIFIED`
- NO_PRODUCTION / NO_COMMIT / NO_PUSH / NO_MERGE

## Workflow step description Safe HTML (2026-08-22)

- Additive `description_format` (`plain_text`|`safe_html`) on workflow step JSON/manifest; FE shared DOMPurify renderer.
- Pointer: FE `../cobo_web_design/docs/ai-cache/workflow-step-safe-html-2026-08-22/`
- Verdict: `WORKFLOW_STEP_SAFE_HTML_DEV_VERIFIED` / Live Activate `NOT_VERIFIED`
- NO_PRODUCTION / NO_COMMIT / NO_PUSH / NO_MERGE

## Recurring disclosure Effective T Scheduling V1 (2026-08-21)

- Pointer: FE `../cobo_web_design/docs/ai-cache/recurring-disclosure-effective-t-2026-08-21/`
- BE: effective schedule resolver, worker seed, clear override, submitted_at/open_at migration 0131
- Verdict: implemented + DEV deploy; wait confirmation
- NO_PRODUCTION / NO_COMMIT / NO_PUSH

## Recurring disclosure Effective T - reconciliation STOP (2026-08-21)

- Pointer: FE `../cobo_web_design/docs/ai-cache/recurring-disclosure-effective-t-2026-08-21/`
- Verdict: wait PO (inclusive vs exclusive Due, cutoff, legacy T relabel); no implement yet
- NO_CODE / NO_DB / NO_DEPLOY / NO_COMMIT / NO_PUSH

## Recurring disclosure business contract extraction (2026-08-21)

- Pointer: FE `../cobo_web_design/docs/ai-cache/recurring-disclosure-business-contract-extraction-2026-08-21.md`
- Verdict: `RECURRING_DISCLOSURE_BUSINESS_CONTRACT_EXTRACTION_COMPLETE` - wait PO; `IMPLEMENTATION_SAFE_TO_START=false`
- NO_CODE / NO_DB / NO_DEPLOY / NO_COMMIT / NO_PUSH

## Template cycle / periodicity vs deadline - source audit (2026-08-21)

- Pointer: FE `../cobo_web_design/docs/ai-cache/template-cycle-deadline-source-audit-2026-08-21.md`
- Verdict `TEMPLATE_CYCLE_DEADLINE_SOURCE_AUDIT_COMPLETE` / superseded for scheduling by recurring-disclosure extraction
- NO_CODE / NO_DB / NO_DEPLOY / NO_COMMIT / NO_PUSH

## Workflow publish readiness + -ang l-n Portal CTA (2026-08-21)

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

## Template clone / create-from-existing - analysis plan (2026-08-21)

- Pointer: FE `docs/ai-cache/template-clone-create-from-existing-analysis-2026-08-21/`
- Verdict `TEMPLATE_CLONE_CREATE_FROM_EXISTING_PLAN_READY` (analysis only; no BE product code this run)
- NO_CODE / NO_DB / NO_DEPLOY / NO_COMMIT / NO_PUSH

## Company workflow override contract (2026-08-20)

- COMPANY_OVERRIDE_ACTIVE > ResolveCMSDefaultWorkflow; draft/activate isolation; reset fallback
- Evidence (FE pack): ../cobo_web_design/docs/ai-cache/company-workflow-override-contract-2026-08-20/
- Verdict `COMPANY_WORKFLOW_OVERRIDE_CONTRACT_DEV_VERIFIED`
- NO_PRODUCTION / NO_PUSH

?## Phase 1 - Centralize Workflow Authority (2026-08-20)

- Pointer: FE `docs/ai-cache/phase1-centralize-workflow-authority-2026-08-20/`
- BE: `ResolveCMSDefaultWorkflow` + Activate/GetEffective adapters; `GetActiveGlobalWorkflow`
- Verdict `PHASE1_CENTRALIZED_WORKFLOW_AUTHORITY_DEV_VERIFIED`; api/worker `2026-08-20T07:29:19Z`
- NO_PRODUCTION / NO_PUSH

## Deadline alert visibility - active template only (2026-08-18)

- Pointer: FE `docs/ai-cache/deadline-alert-active-template-filter-2026-08-18/`
- ListRows INNER JOIN current `active_version_no > 0`; no business delete
- Verdict `DEV_DEADLINE_ALERT_ACTIVE_TEMPLATE_FILTER_VERIFIED`; api `2026-08-18T08:47:17Z`
- NO_PRODUCTION / NO_PUSH

## Deadline alert cleanup before 2026-08-17 (DEV, blocked)

- Pointer: FE `docs/ai-cache/deadline-alert-cleanup-before-2026-08-17/`
- Entity = `disclosure_records`; no delete executed
- `BLOCKED_DEADLINE_ALERT_ENTITY_IS_BUSINESS_RECORD`
- NO_PRODUCTION / NO_PUSH

## Workflow step reminder - email business context (2026-08-18)

- Pointer: FE `docs/ai-cache/workflow-step-reminder-rule-engine-2026-08-17/email-business-context-audit-2026-08-18/`
- Reminder CTA/deadline fix; company source = proposal/instance; api+worker `2026-08-18T05:12:07Z`
- `BLOCKED_EMAIL_EVIDENCE_UNAVAILABLE=CLOSED`; 2 paused blockers unchanged
- NO_PRODUCTION / NO_PUSH

## Workflow step reminder - E1 no-head fallback (2026-08-18)

- Pointer: FE `docs/ai-cache/workflow-step-reminder-rule-engine-2026-08-17/e1-no-head-fallback-2026-08-18/`
- Verdict `WORKFLOW_ALERT_E1_NO_HEAD_FALLBACK_DEV_VERIFIED`; api+worker `2026-08-18T03:46:59Z`
- NOT `WORKFLOW_STEP_REMINDER_RULE_DEEP_SMOKE_DEV_READY`; 3 paused blockers unchanged
- NO_PRODUCTION / NO_PUSH

## Workflow step reminder - recipient authority A/B/C/D (2026-08-18)

- Pointer: FE `docs/ai-cache/workflow-step-reminder-rule-engine-2026-08-17/recipient-authority-verify-2026-08-18/`
- Verdict `WORKFLOW_ALERT_RECIPIENT_AUTHORITY_DEV_VERIFIED`; api+worker `2026-08-18T02:40:45Z`
- NOT `WORKFLOW_STEP_REMINDER_RULE_DEEP_SMOKE_DEV_READY`; 3 paused blockers unchanged
- NO_PRODUCTION / NO_PUSH

## Workflow step reminder - department alert Step 1 (2026-08-18)

- Pointer: FE `docs/ai-cache/workflow-step-reminder-rule-engine-2026-08-17/department-alert-validation-2026-08-18/`
- Step 1 PARTIAL (mailbox Level 3 missing); snapshot-first recipient fix uncommitted; DEV api+worker `2026-08-18T01:33:28Z`
- Plan only: `19-three-blocker-resolution-plan.md`
- NOT `WORKFLOW_STEP_REMINDER_RULE_DEEP_SMOKE_DEV_READY`
- NO_PRODUCTION / NO_PUSH

## Workflow step reminder - migration recovery (2026-08-17)

- Pointer: FE pack `workflow-step-reminder-rule-engine-2026-08-17/` (`76`-`85` + `dev-smoke-custom-default/55+`)
- 0129 applied; `MIGRATION_REQUIRED=true` / `MIGRATION_IMPLEMENTED=true`; source-ready restored
- Remaining: `BLOCKED_EMAIL_EVIDENCE_UNAVAILABLE` (SMTP Gmail, not Mailpit)
- NOT `WORKFLOW_STEP_REMINDER_RULE_DEEP_SMOKE_DEV_READY`
- NO_PRODUCTION / NO_PUSH

## Workflow step reminder - Phase 3-5 DEV (2026-08-17)

- Pointer: FE pack `cobo_web_design/docs/ai-cache/workflow-step-reminder-rule-engine-2026-08-17/dev-smoke-custom-default/`
- Phase 3-4 PASS; Phase 5 ENUM blocker repaired by 0129 (see recovery entry)
- NOT `WORKFLOW_STEP_REMINDER_RULE_DEEP_SMOKE_DEV_READY`
- NO_PRODUCTION / NO_PUSH

## Workflow step reminder - Phase 2D source-ready (2026-08-17)

- Pointer: FE pack `cobo_web_design/docs/ai-cache/workflow-step-reminder-rule-engine-2026-08-17/` (`62`-`75`)
- Verdict **WORKFLOW_STEP_REMINDER_RULE_ENGINE_READY**
- Docker: `build api` + `build worker` + FE `run --rm --no-deps web npm ci && npm run build`
- NO_MIGRATION / NO_DEPLOY / NO_PRODUCTION / NO_PUSH; Phase 3 awaits confirm

## Workflow step reminder - Phase 2C BE runtime (2026-08-17)

- Pointer: FE pack `cobo_web_design/docs/ai-cache/workflow-step-reminder-rule-engine-2026-08-17/` (`46`-`61`)
- Verdict **WORKFLOW_STEP_REMINDER_PHASE2C_RUNTIME_ENGINE_READY**
- BE product: resolver + Path B `documents_json` persist + snapshot + due-minus engine
- NO_MIGRATION / NO_DEPLOY / NO_PRODUCTION / NO_PUSH; Phase 2D awaits confirm

## Workflow step reminder - Phase 2B CMS FE (2026-08-17)

- Pointer: FE pack `cobo_web_design/docs/ai-cache/workflow-step-reminder-rule-engine-2026-08-17/` (`28`-`29`, `37`-`45`)
- Verdict **WORKFLOW_STEP_REMINDER_PHASE2B_CMS_FE_READY**
- BE product source **unchanged**; runtime authority still planned `internal/disclosure/app/workflow_step_reminder_rule.go` (Phase 2C)
- NO_BACKEND_SOURCE_CHANGE / NO_MIGRATION / NO_DEPLOY / NO_PRODUCTION / NO_PUSH

## Workflow step reminder - Phase 2A contract lock (2026-08-17)


- Pointer: FE pack `cobo_web_design/docs/ai-cache/workflow-step-reminder-rule-engine-2026-08-17/` (`24`-`27`)
- Verdict **WORKFLOW_STEP_REMINDER_PHASE2A_CONTRACT_LOCKED**
- BE: no product source this phase; planned runtime authority `internal/disclosure/app/workflow_step_reminder_rule.go` (Phase 2C)
- NO_BACKEND_SOURCE_CHANGE / NO_MIGRATION / NO_DEPLOY / NO_PRODUCTION / NO_PUSH

## CMS irregular ? ad_hoc template persistence fix (2026-08-17)

- Pointer: FE pack `cobo_web_design/docs/ai-cache/cms-irregular-adhoc-template-persistence-fix-2026-08-17/`
- Verdict **CMS_IRREGULAR_ADHOC_TEMPLATE_PERSISTENCE_FIX_READY** (FE-only)
- BE: no source/DTO/validation/alert-engine change required
- NO_BACKEND_SOURCE_CHANGE / NO_MIGRATION / NO_DEV_DEPLOY / NO_PRODUCTION / NO_PUSH



## CMS template ? Enterprise abnormal alert post-fix deep smoke (2026-08-17)

- Pointer: FE pack `cobo_web_design/docs/ai-cache/cms-template-abnormal-alert-deep-smoke-2026-08-17/post-fix/`
- Verdict **CMS_TEMPLATE_TO_ENTERPRISE_ABNORMAL_ALERT_DEEP_SMOKE_DEV_READY**
- BE unchanged this closeout; API/worker StartedAt `2026-08-14T03:39:45Z`
- NO_BACKEND_DEPLOY / NO_WORKER_RESTART / NO_MIGRATION / NO_PRODUCTION / NO_PUSH

## Manual QR company package activation DEV (2026-08-14)

- Verdict **COBO_MANUAL_QR_COMPANY_PACKAGE_ACTIVATION_DEV_READY**
- `make deploy-be` recreates api+worker; no migrate; live lookup `SELECT company_code`
- Evidence (FE pack): `cobo_web_design/docs/ai-cache/manual-qr-company-package-activation-2026-08-14/` (`24`-`65`)
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

## CMS - template daily/weekly cycle DEV (2026-08-14)

- Verdict **CMS_TEMPLATE_DAILY_WEEKLY_CYCLE_DEV_READY**
- `make deploy-be` api+worker; worker SQL includes daily/weekly; PERIODIC_SEEDING_ENABLED=false
- Evidence (FE pack): `cobo_web_design/docs/ai-cache/cms-template-daily-weekly-cycle-2026-08-14/`
- NO_MIGRATION / NO_PRODUCTION / NO_PUSH

## Ad-hoc proposal tracking - T5 DEV deploy + E2E (2026-08-10)


- Verdict **ADHOC_PROPOSAL_TRACKING_DEV_READY**
- BE T1+T3 + FE T2+T3 deployed DEV; hotfix: propose-only detail no longer hard-redirects on admin org-directory 403
- Evidence: `adhoc-proposal-tracking-discoverability-2026-08-10/` (`113`-`148`)
- NO_PRODUCTION - STOP at DEV
## Ad-hoc proposal tracking - T4 integration (2026-08-10)

- Verdict **T4_ADHOC_PROPOSAL_TRACKING_INTEGRATION_READY**
- Docker API build parity **PASS**; worker redeploy not required for tracking
- Evidence (FE pack): `cobo_web_design/docs/ai-cache/adhoc-proposal-tracking-discoverability-2026-08-10/` (`91`-`112`)
- No migration / no deploy - await T5 confirm

## Ad-hoc proposal tracking - T3 detail runtime (2026-08-10)

- Verdict **T3_ADHOC_PROPOSAL_TRACKING_DETAIL_READY**
- Mode **T3_BE_DETAIL_PROJECTION_ADDED**: `tracking` on GetProposal via workflow instance/tasks
- Evidence (FE pack): `cobo_web_design/docs/ai-cache/adhoc-proposal-tracking-discoverability-2026-08-10/` (`72`-`90`)
- Quality: adhoc tests + vet + api build PASS; docker api build **BLOCKED** (daemon)
- No migration / no deploy - await T4/T5 confirm

## Ad-hoc proposal tracking - T1 backend (2026-08-10)

- Verdict **T1_ADHOC_PROPOSAL_TRACKING_BACKEND_READY**
- Self-detail + `scope=my`; company-wide list still `ad_hoc_alert.read`
- Evidence (FE pack): `cobo_web_design/docs/ai-cache/adhoc-proposal-tracking-discoverability-2026-08-10/` (`44`-`56`)
- Quality: adhoc tests + vet + api build + docker api build PASS
- No migration / no FE / no deploy - await T2 confirm

## Ad-hoc proposal tracking - T0 contract (2026-08-10)

- Verdict **T0_ADHOC_PROPOSAL_TRACKING_PRODUCT_CONTRACT_READY**
- **`T0_BACKEND_SELF_READ_REQUIRED=true`**: creator self-detail + `scope=my`; company-wide list still `ad_hoc_alert.read`
- Evidence (FE pack): `cobo_web_design/docs/ai-cache/adhoc-proposal-tracking-discoverability-2026-08-10/` (`30`-`43`)
- No BE source/migration/deploy - await confirm → T1

## Ad-hoc proposal tracking discoverability - plan (2026-08-10)

- Verdict **ADHOC_PROPOSAL_TRACKING_IMPLEMENTATION_PLAN_READY** (FE evidence pack)
- List/Get APIs already exist; primary gap is FE IA/nav + optional authz for propose-only creators
- Evidence (FE): `cobo_web_design/docs/ai-cache/adhoc-proposal-tracking-discoverability-2026-08-10/`
- No BE source/migration/deploy - await confirm

## Ad-hoc proposal multi-assignee - M4 DEV (2026-08-10)

- Verdict **ADHOC_PROPOSAL_MULTI_ASSIGNEE_DEV_READY**
- Migration `0128` applied DEV only; BE api+worker + FE deployed; authenticated E2E PASS
- Evidence: `docs/ai-cache/adhoc-proposal-multi-assignee-2026-08-10/` (`93`-`130`) + results.m4
- Rollback: requires v3 task drain/compat plan (not blind app rollback)
- NO_PRODUCTION - STOP at DEV

## Ad-hoc proposal multi-assignee - M3 FE (2026-08-10)

- Verdict **M3_ADHOC_PROPOSAL_MULTI_ASSIGNEE_FRONTEND_READY** (FE pointer)
- BE source unchanged this phase; evidence pack `76`-`92` on FE (+ results.m3)
- Next: M4 migration apply + coordinated deploy

## Ad-hoc proposal multi-assignee - M2 runtime + recipients (2026-08-10)

- Verdict **M2_ADHOC_PROPOSAL_MULTI_ASSIGNEE_RUNTIME_READY**
- Migration `0128` source only (nullable singular + workflow_task_assignees); NOT applied
- Runtime: v3 one logical task + relation; ANY completion; Personal Ops / deadlinealerts / reminder readers model-aware
- Evidence: `docs/ai-cache/adhoc-proposal-multi-assignee-2026-08-10/` (`56`-`75`) + FE mirror pack
- No FE / no DEV deploy / no Production - await M3 confirm

## Ad-hoc proposal multi-assignee - M1 backend contract (2026-08-10)

- Verdict **M1_ADHOC_PROPOSAL_MULTI_ASSIGNEE_BACKEND_CONTRACT_READY**
- Source in this repo: workflow v3 snapshot + head resolver + submit normalize; v3 materialize guarded
- Canonical evidence: `../cobo_web_design/docs/ai-cache/adhoc-proposal-multi-assignee-2026-08-10/` (`40`-`55`)
- No migration / no DEV deploy / no Production - await M2 confirm

## Ad-hoc proposal multi-assignee - M0 Product contract lock (2026-08-10)

- Verdict **M0_ADHOC_PROPOSAL_MULTI_ASSIGNEE_PRODUCT_CONTRACT_READY** (pointer)
- Canonical evidence: `../cobo_web_design/docs/ai-cache/adhoc-proposal-multi-assignee-2026-08-10/` (`28`-`39`)
- Locked: ANY; schema_version=3; workflow_task_assignees; submit-time head; active-step multi recipients
- Superseded implement start → M1 (see above)

## Ad-hoc proposal multi-assignee + department-head default - audit/plan (2026-08-10)

- Verdict **ADHOC_PROPOSAL_MULTI_ASSIGNEE_PLAN_BLOCKED_PRODUCT_DECISION** (historical pointer)
- Canonical evidence: `../cobo_web_design/docs/ai-cache/adhoc-proposal-multi-assignee-2026-08-10/`
- Docs-only; blocker cleared by M0
- Alert historical: **NO_CURRENTLY_SINGLE_RECIPIENT** (task/inbox singular)

## Ad-hoc proposal deadline day type - Phase D DEV (2026-08-10)

- Verdict **ADHOC_PROPOSAL_DEADLINE_DAY_TYPE_DEV_READY**
- Migration `0127` applied on DEV; api+worker+fe deployed; canonical evidence in cobo_web_design pack `60`-`90`
- NO_PRODUCTION

## Ad-hoc proposal deadline day type - Phase C runtime (2026-08-10)

- Verdict **PHASE_C_ADHOC_PROPOSAL_DEADLINE_DAY_TYPE_RUNTIME_READY**
- Source: `FormatProposalDueDate` + `deadlineengine.AddDaysAfter`; alerts/personalops wired
- Canonical evidence: `../cobo_web_design/docs/ai-cache/adhoc-proposal-deadline-day-type-2026-08-10/` (`44`-`59`)
- Migration 0127 not applied; no DEV/Production deploy - await Phase D

## Ad-hoc proposal deadline day type - Phase B FE (2026-08-10)

- Verdict **PHASE_B_ADHOC_PROPOSAL_DEADLINE_DAY_TYPE_FRONTEND_READY** (pointer)
- Canonical evidence: `../cobo_web_design/docs/ai-cache/adhoc-proposal-deadline-day-type-2026-08-10/`
- No BE source this phase; no deploy

## Ad-hoc proposal deadline day type - Phase A (2026-08-10)

- Verdict **PHASE_A_ADHOC_PROPOSAL_DEADLINE_DAY_TYPE_BACKEND_READY**
- Source: `proposed_deadline_day_type` + migration `0127_*` (not applied)
- Canonical evidence: `../cobo_web_design/docs/ai-cache/adhoc-proposal-deadline-day-type-2026-08-10/`
- No DEV deploy / no Production

# Cursor Skill Pack for Cobo Repos

## Tín hiệu tuân thủ - phải thấy được trong Chat (bắt buộc)

Giống `cobo_web_design/docs/ai-cache/README.md`: mọi câu trả lời có nội dung phải có **dòng đầu** bắt đầu **`[ai-cache]`** + README + file `docs/ai-cache/` đã dùng + skill + **`Mandatory README: đã áp dụng`**. Chi tiết: xem README trong `cobo_web_design` hoặc sao chép mục đó vào repo này nếu làm việc chỉ IAM.

Snippet **“Bắt buộc: tuân thủ docs/ai-cache/README.md…”** (dán đầu prompt) và **lệnh Docker/build hoặc `BLOCKED:`** sau implement: xem **`cobo_web_design/docs/ai-cache/README.md`** và bản siết trong **`.cursor/rules/ai-cache-read-first.mdc`** của repo này.

**Áp dụng tự động (Cursor):** Luật **`.cursor/rules/ai-cache-read-first.mdc`** (`alwaysApply: true`) + **`AGENTS.md`** ở root.

Pack này gồm 2 bộ cấu hình:
- `cobo_web_design/.cursor/...`
- `cobo_iam_services/.cursor/...`

## Cách dùng
1. Copy thư mục `.cursor` trong từng repo vào đúng project tương ứng.
2. Giữ các `rules/*.mdc` để luôn bật guardrails kiến trúc.
3. Dùng Agent trong Cursor và prompt theo skill tương ứng.
4. Với task lớn, bắt đầu bằng `system-design-feature`.
5. Trước khi hoàn tất, luôn chạy `premerge-system-review`.

## Prompt mặc định nên dán cho hầu hết mọi câu hỏi

Dùng prompt này như prompt khởi đầu gần như mỗi lần hỏi Cursor.

```text
Use the relevant project skill for this task.
First identify the architectural boundary, affected layers, domain invariants, failure modes, validation strategy, and test scope before writing code.
Preserve backward compatibility unless explicitly asked otherwise.
Prefer minimal, reviewable diffs.
Do not skip loading/error/empty states on frontend.
Do not skip validation, authorization, idempotency, migration safety, or observability on backend.
Before marking the task done, run a pre-merge review and report risks, gaps, and verification steps.
```




## Ad-hoc proposal custom workflow - Phase 3.6 multi-step runtime (2026-08-07)

- Verdict **PHASE_3_6_ADHOC_PROPOSAL_MULTI_STEP_RUNTIME_READY**
- Evidence: `docs/ai-cache/adhoc-proposal-custom-workflow-contract-2026-08-07/` (`64`-`76`)
- Lazy one-active-task chain; instance completes only after final frozen step; **NO_DEV_DEPLOY**

## Ad-hoc proposal deadline day type - plan (2026-08-10)

- Verdict **ADHOC_PROPOSAL_DEADLINE_DAY_TYPE_IMPLEMENTATION_PLAN_READY** (pointer)
- Canonical evidence: `../cobo_web_design/docs/ai-cache/adhoc-proposal-deadline-day-type-2026-08-10/`
- Docs-only FE commit; no BE source this phase

## Ad-hoc proposal custom workflow - Phase 3.5 assignment convergence (2026-08-07)

- Verdict **PHASE_3_5_ADHOC_PROPOSAL_ASSIGNMENT_CONTRACT_READY**
- Submit-time `ValidateWorkflowForSubmit` for schema v2; draft incomplete still allowed; evidence under FE `docs/ai-cache/adhoc-proposal-custom-workflow-contract-2026-08-07/` (`54`-`63`)
- **NO_DEV_DEPLOY** until Phase 4

## Ad-hoc proposal custom workflow - Phase 3 runtime (2026-08-07)

- Verdict **PHASE_3_ADHOC_PROPOSAL_CUSTOM_WORKFLOW_RUNTIME_READY**
- Source in this repo; evidence canonical under sibling FE `docs/ai-cache/adhoc-proposal-custom-workflow-contract-2026-08-07/` (`40`-`53`)
- `runtimeV2Implemented=true`; assignment `V2_DIRECT_ASSIGNEE_REQUIRED`; **NO_DEV_DEPLOY** until Phase 4
- Next: Phase 3.6 multi-step chain complete - await Phase 4

## Ad-hoc proposal custom workflow - Phase 1 (2026-08-07)

- Verdict **PHASE_1_ADHOC_PROPOSAL_CUSTOM_WORKFLOW_BACKEND_FOUNDATION_READY**
- Source in this repo; evidence canonical under sibling FE `docs/ai-cache/adhoc-proposal-custom-workflow-contract-2026-08-07/`
- `NO_MIGRATION_REQUIRED`; Phase 3 runtime landed (see above)
- Next: await Phase 4

## Company department metric - Phase 2 DEV (2026-08-06)

- Verdict **PHASE_2_COMPANY_DEPARTMENT_METRIC_DEV_READY** (pointer)
- BE commit deployed: `a9d03fb`
- Canonical evidence: `../cobo_web_design/docs/ai-cache/company-department-metric-2026-08-06/`
- Status: stop at DEV - no Production

## Company department metric - Phase 1 (2026-08-06)

- Verdict **PHASE_1_COMPANY_DEPARTMENT_METRIC_SOURCE_READY** (pointer)
- Additive `department_count` on company profile / `GetCompanyPlatform`
- Canonical evidence: `../cobo_web_design/docs/ai-cache/company-department-metric-2026-08-06/`
- Next: await Phase 2 DEV deploy confirm

## Company Premium End-to-End - Phase 8 final handoff (2026-08-04)

- Verdict **COMPANY_PREMIUM_DEV_READY**: `docs/ai-cache/company-premium-implementation-2026-08-04/` (`36`-`46`, `results.json`)
- Lineage + contract + nginx + security + rollback reconciled; Phase 8 docs-only; Production untouched
- FE mirror: `../cobo_web_design/docs/ai-cache/company-premium-fe-dev-2026-08-04/` (`21`-`23`)

## Company Premium implementation - Phase 5 Backend DEV (2026-08-04)

- Verdict **PHASE_5_BACKEND_DEV_READY**: `docs/ai-cache/company-premium-implementation-2026-08-04/` (`20`-`35`)
- DEV migrate 0125 + seed; live concurrency PASS_DEV; API deployed `dd0ff1e`; smoke/authz/FE compat PASS
- Superseded by Phase 8 final verdict above

## Company Premium implementation - Phase 4 backend quality (2026-08-04)

- Verdict **PHASE_4_BACKEND_QUALITY_READY**: `docs/ai-cache/company-premium-implementation-2026-08-04/` (`14`-`19`)
- PatchOwnCompany plan-before-mutation fix; STRICT/security/migration static gates; task failures 0
- Phase 5 complete - see entry above

## Company Premium implementation - Phase 3 API exposure (2026-08-04)

- Verdict **PHASE_3_API_EXPOSURE_READY**: `docs/ai-cache/company-premium-implementation-2026-08-04/` (`10`-`13`)
- Additive `plan` on GetOwnCompany + `/me/companies`; STRICT enrichment errors; batch reader; no FE/deploy/migrate-DEV
- Open risk carried: `MYSQL_CONCURRENCY_VALIDATION_PENDING_PHASE_5` (not Phase 3 gate)
- Phase 4 complete - see entry above

## Company Premium implementation - Phase 2 shared Reader (2026-08-04)

- Verdict **PHASE_2_SHARED_READER_READY** (+ `MYSQL_CONCURRENCY_VALIDATION_PENDING_PHASE_5`): `docs/ai-cache/company-premium-implementation-2026-08-04/` (`06`-`09`)
- Shared `companyplan.Service`; 0126 retracted → DEV seed; parent-company FOR UPDATE; no API/FE/deploy
- Phase 3 API exposure complete - see entry above

## Company Premium implementation - Phase 1 domain foundation (2026-08-04)

- Verdict **PHASE_1_DOMAIN_FOUNDATION_READY**: `docs/ai-cache/company-premium-implementation-2026-08-04/` (`03`-`05`)
- Package `internal/subscription/companyplan`; schema `0125`; DEV fixtures moved to seed in Phase 2; tests PASS; no API/deploy/migrate-DEV

## Company Premium implementation - Phase 0 contract lock (2026-08-04)

- Verdict **PHASE_0_CONTRACT_LOCKED**: `docs/ai-cache/company-premium-implementation-2026-08-04/`
- Case C approved via user instruction 2026-08-04; `company_subscriptions` SoT; `plan: null` no-plan; badge = PREMIUM+ACTIVE+COMPANY_SUBSCRIPTION
- Awaiting user confirmation before Phase 1 (domain + migration)

## Company Premium Backend implementation plan (2026-08-03)

- Verdict **COMPANY_PREMIUM_IMPLEMENTATION_PLAN_READY**: `docs/ai-cache/company-premium-backend-implementation-plan-2026-08-03/`
- Deep audit @ `9284a31`; Case B paid-plan source; A+B API `RECOMMENDED_PENDING_APPROVAL`; exact file map + phases; no source/migration/deploy

## Operational Dashboard real KPI aggregation (2026-08-03)

- Verdict **DASHBOARD_REAL_KPI_IMPLEMENTATION_READY** - pointer `docs/ai-cache/dashboard-kpi-real-aggregation-2026-08-03/`; canonical pack in FE sibling
- DEV deploy smoke **PASS_WITH_DATA_LIMITATIONS** - see `20-dev-deploy-pointer.md` + FE `results.json`
- Remap overview KPIs + completed_at read-only; no migration/deploy

## Workflow alert tasks 500 + source UX (DEV)

- Verdict **PASS_TASKS_API_AND_WORKFLOW_SOURCE_UX** (2026-07-30): `docs/ai-cache/workflow-alert-tasks-500-and-source-ux-remediation-2026-07-30/`
- T500-RC4 NULL department Scan; CMS dual-source Global Template v1 · 4 steps; no rematerialize

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

## Prompt tái sử dụng theo từng tình huống

### 1) Khi xây feature mới từ đầu

```text
Use system-design-feature first, then switch to the relevant repo-specific implementation skill.
Before coding, define the objective, user flow, domain invariants, API contract, UI states, data flow, failure modes, rollout approach, and test plan.
Only then implement with minimal and reviewable diffs.
```

### 2) Khi làm feature frontend trong `cobo_web_design`

```text
Use the frontend skill that best matches this task.
Follow the vertical slice structure: route -> screen -> feature components -> hooks/services -> types.
Do not mix route concerns, fetching concerns, and presentation concerns in one large file.
Handle loading, error, empty, success, disabled, and invalid-param states explicitly.
Add focused Vitest/testing-library coverage for the core user-visible behavior.
```

### 3) Khi làm feature backend trong `cobo_iam_services`

```text
Use the backend skill that best matches this task.
Keep boundaries clear: handler -> service/usecase -> repository -> external systems.
Define request/response contract, validation rules, authorization rules, transaction boundaries, cache impact, retry/idempotency considerations, and test matrix before coding.
Do not hide security, migration, or data consistency risks.
```

### 4) Khi sửa bug

```text
Use the relevant debugging or repo-specific skill.
First restate the symptom, expected behavior, actual behavior, likely root causes, and the most probable failure path from code.
Fix the root cause with the smallest safe change, then add regression protection and list any remaining uncertainty.
```

### 5) Khi review trước merge

```text
Run premerge-system-review.
Audit requirement coverage, architectural fit, frontend state completeness, validation completeness, API/contract consistency, auth/security risks, data consistency risks, migration/deployment risks, observability gaps, and missing regression tests.
Group findings into critical, important, and nice-to-have.
```

## Prompt siêu ngắn để ghim cố định

Nếu bạn muốn một prompt ngắn hơn để dùng liên tục:

```text
Use the relevant project skill. Think in layers, define contracts first, handle failure modes explicitly, keep changes minimal, and do a system-level review before done.
```

## Mẹo dùng thực tế
- Với task mơ hồ: luôn bắt đầu bằng prompt feature mới.
- Với task chỉ chạm UI: dùng prompt frontend.
- Với task auth/API/data: dùng prompt backend.
- Với bug khó: dùng prompt sửa bug.
- Với PR sắp xong: dùng prompt review trước merge.

## Prompt khuyên dùng cũ

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

Áp dụng bắt buộc cho mọi prompt liên quan `cobo_web_design` và/hoặc `cobo_iam_services`:

1. Trước mọi bước, đọc `docs/ai-cache/README.md` và toàn bộ context tái sử dụng trong `docs/ai-cache/`.
2. Thứ tự ưu tiên khi có xung đột:
   - `docs/ai-cache/README.md`
   - các file còn lại trong `docs/ai-cache/`
   - project rules
   - docs/pattern cũ trong repo
3. Chọn skill phù hợp; nếu task chạm cả 2 repo thì dùng `integration-cross-repo`.
4. Với feature mới: bắt buộc contract-first trước khi code (contract + request/response/error matrix + FE mapping + BE expectations + failure modes + rollout risks + validation plan).
5. Với task implement:
   - diff nhỏ, dễ review
   - kết thúc bằng `premerge-system-review`
   - sau mỗi cycle có thay code phải rerun fresh Docker build cho services bị ảnh hưởng
   - không coi là xong cho tới khi Docker build mới nhất đã chạy và báo kết quả
6. Với task phân tích/review:
   - không sửa code nếu chưa được yêu cầu explicit
   - phân tích dựa trên code/docs thực tế, không phỏng đoán
7. Sau mỗi task (implement hoặc understand), ghi tóm tắt tái sử dụng vào `docs/ai-cache/` theo format ngắn, nhất quán:
   - task type
   - objective/question
   - implemented/discovered
   - affected repos/files/modules
   - contracts/behaviors/constraints/decisions
   - build/verification result (nếu có)
   - remaining gaps/risks/next steps
## Workflow Configuration load 404 (DEV)

- Canonical audit (FE pack): `../cobo_web_design/docs/ai-cache/workflow-config-load-404-audit-2026-07-30/`
- Remediation (FE pack): `../cobo_web_design/docs/ai-cache/workflow-config-load-404-remediation-2026-07-30/` - DEV flag ON, isolation PASS
- **ROOT_CAUSE_CONFIRMED** RC-4 then remediated: `WORKFLOW_VERSIONING_ENABLED` was OFF → routes not registered
- Mirror note: `reusable-task-updates.md` (2026-07-30)

## Current-month periodic deadline alert E2E (DEV)

- Canonical FE pack: `../cobo_web_design/docs/ai-cache/current-month-periodic-deadline-alert-e2e-2026-07-30/`
- Verdict: **BLOCKED_PERIODIC_MATERIALIZATION** - QA active; seeding remains OFF
- Materialization later unblocked by guarded one-shot pack above

## Q2 Financial Report Deadline Alert E2E (DEV)

- Canonical (FE pack): `../cobo_web_design/docs/ai-cache/q2-financial-report-deadline-alert-e2e-2026-07-30/`
- Verdict: **BLOCKED_DEADLINE_CONTRACT** - no PERIOD_END in deadline engine/CMS; seeding remains OFF

## Deadline alert - active report missing? (DEV)

- Canonical audit (FE pack): `../cobo_web_design/docs/ai-cache/deadline-alert-active-report-audit-2026-07-30/`
- Verdict: **EXPECTED_BEHAVIOR_CONFIRMED** (RC-1) - `bao-cao-tai-chinh-quy-2` on `c_001` returns `PENDING_CONFIRM`, not open OVERDUE
- Flags on DEV: `PERIODIC_SEEDING_ENABLED=false`, `DEADLINE_ENGINE_V2=false` (shadow true)

## Legal Basis Phase 12.6 (IAM mirror)

- Phase 12.6A: operational `PASS_READ_ONLY_DRY_RUN`; governance `FAIL_SCOPE_CREEP` - see plan folder `phase-12-6a-scope-exception.md`
- Phase 12.6B-Plan: `BACKFILL_PLAN_READY` - `phase-12-6b-plan-handoff.md` (docs only; no apply)

