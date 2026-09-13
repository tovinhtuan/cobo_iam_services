const fs = require('fs');
const path = require('path');

const dirFE = path.join(
  'c:/Users/tvttt/OneDrive/Desktop/cobo/cobo_web/cobo_web_design/docs/ai-cache/tenant-workflow-step-evidence-b3-complete-gate-2026-09-13',
);
const dirBE = path.join(
  'c:/Users/tvttt/OneDrive/Desktop/cobo/cobo_web/cobo_iam_services/docs/ai-cache/tenant-workflow-step-evidence-b3-complete-gate-2026-09-13',
);
fs.mkdirSync(dirFE, { recursive: true });
fs.mkdirSync(dirBE, { recursive: true });

const smokePath = path.join(dirFE, 'dev-smoke-result.json');
const smoke = JSON.parse(fs.readFileSync(smokePath, 'utf8'));

const files = {
  '00-context.md': `# B3 Complete Gate
IMPLEMENTATION B3 ONLY — block Complete Step when required B1 snapshots lack ACTIVE B2 fulfillments.
BE ONLY. NO FE. NO DB migration. NO B2 file API change.
`,
  '01-v1-contract-reference.md': `Accepted: tenant-workflow-step-evidence-v1-contract-plan-2026-09-13
REQUIRED_DOCUMENT_VALIDATION_AUTHORITY=BE; LIVE_DOCUMENTS_USED_FOR_COMPLETE_VALIDATION=false
`,
  '02-b1-reference.md': `B1 accepted: workflow_step_document_requirement_snapshots immutable; CreateWorkflowInstanceInternal same TX; no legacy backfill.
`,
  '03-b2-reference.md': `B2 accepted: ACTIVE|SUPERSEDED|DELETED; max 10 ACTIVE; lock snap→step; mutation deadline.confirm+current+!completed.
`,
  '04-worktree-baseline.md': `BE branch recovery/lost-changes-audit-20260717-153324 HEAD c517894 + B1/B2/B3 delta.
PREEXISTING_USER_CHANGES_PRESERVED=true. FE_SOURCE_CHANGED=false. DB_MIGRATION_CREATED=false.
`,
  '05-complete-step-call-chain.md': `COMPLETE_STEP_HANDLER=deadlinealerts/transport/http completeDeadlineStep
COMPLETE_STEP_SERVICE=deadlinealerts/app CompleteDeadlineStep
COMPLETE_STEP_TRANSACTION_BOUNDARY=deadlinealerts/infra/mysql UpsertStepCompleted BeginTx
CURRENT_STEP_VALIDATION_SOURCE=ComputeDeadlineSteps CurrentStepCode vs req.StepCode (pre-TX)
`,
  '06-validation-position.md': `VALIDATION_BEFORE_STEP_COMPLETION_MUTATION=true
Order in UpsertStepCompleted: lock snaps → lock step → if already completed commit → ValidateRequiredDocumentFulfillmentLocked → INSERT completed → commit
VALIDATION_AND_COMPLETE_SAME_TX=true
`,
  '07-required-document-validator.md': `REQUIRED_DOCUMENT_VALIDATOR=workflowfulfillment.EvaluateMissingRequiredDocuments + mysql.ValidateRequiredDocumentFulfillmentLocked
Wired from UpsertStepCompleted only (Complete path). Mark Incomplete unchanged.
`,
  '08-snapshot-authority.md': `COMPLETE_VALIDATION_USES_SNAPSHOT_ONLY=true
Load: workflow_step_document_requirement_snapshots WHERE company+instance+step ORDER BY id FOR UPDATE
No live documents[] / CMS / override fallback.
`,
  '09-active-fulfillment-authority.md': `ACTIVE_FULFILLMENT_LOOKUP_STRATEGY=GROUP BY requirement_snapshot_id COUNT WHERE lifecycle_status=ACTIVE (company+instance+step)
REQUIRED_REQUIREMENT_FULFILLED_WHEN=ACTIVE_COUNT_GTE_1
SUPERSEDED/DELETED/template_file_id do not count. B3_N_PLUS_ONE_QUERY=false
`,
  '10-legacy-contract.md': `ZERO_SNAPSHOT_BEHAVIOR=LEGACY_PRESERVE_EXISTING
ZERO_SNAPSHOT_LIVE_FALLBACK=false
EXISTING_OCCURRENCE_BACKFILL=false
`,
  '11-lock-strategy.md': `COMPLETE_GATE_LOCK_STRATEGY=lock all step snapshots id ASC FOR UPDATE → lock step state FOR UPDATE → count ACTIVE → mutate complete
B2 Upload/Replace: single snap FOR UPDATE → step FOR UPDATE
B2 Delete: step FOR UPDATE → file FOR UPDATE (no snap)
COMPLETE_AND_B2_LOCK_ORDER_COMPATIBLE=true (snap-before-step for Complete/Upload/Replace; Delete waits on step)
`,
  '12-complete-delete-race.md': `COMPLETE_DELETE_REQUIRED_FILE_RACE_SAFE=true
Valid: Complete wins → Delete sees completed; or Delete wins → Complete sees 0 ACTIVE → 422
Memory race TestB3_Race_CompleteVsDeleteLastFile PASS
`,
  '13-complete-upload-race.md': `COMPLETE_UPLOAD_RACE_SAFE=true — TestB3_Race_CompleteVsUpload PASS
`,
  '14-complete-replace-race.md': `COMPLETE_REPLACE_RACE_SAFE=true — replace atomic under mutex/TX; TestB3_Race_CompleteVsReplace PASS
`,
  '15-error-contract.md': `APPERR_CODE_ADDED=true CodeWorkflowStepRequiredDocumentMissing
REQUIRED_DOCUMENT_ERROR_HTTP_STATUS=422
MISSING_REQUIREMENTS_RETURN_ALL=true
`,
  '16-error-payload.md': `MISSING_REQUIREMENT_ORDER=ordinal ASC, source_doc_id ASC, id ASC
MISSING_REQUIREMENT_PAYLOAD_FIELDS=requirement_snapshot_id,source_doc_id,name
No storage_key / SQL / paths. httpx WriteError maps Details.
`,
  '17-validator-tests.md': `B3 evaluate + memory gate tests PASS (required/optional/empty/all-missing/template/multi-active)
`,
  '18-legacy-tests.md': `B3_LEGACY_NO_SNAPSHOT_PRESERVES_COMPLETE_TEST=PASS
B3_NO_LIVE_DOCUMENT_FALLBACK_TEST=PASS
`,
  '19-atomicity-tests.md': `B3_BLOCKED_COMPLETE_ZERO_MUTATION_TEST=PASS
B3_SUCCESS_COMPLETE_REGRESSION_TEST=PASS
COMPLETE_RETRY_AFTER_UPLOAD_SUPPORTED=true
`,
  '20-concurrency-tests.md': `Delete/Upload/Replace race tests PASS; go test -race ./internal/workflowfulfillment/ PASS
NO_NEW_B3_DEADLOCK=true
`,
  '21-api-error-tests.md': `B3_ERROR_CODE/HTTP_STATUS/PAYLOAD/ORDER/NO_INTERNAL_LEAK PASS
`,
  '22-security-tests.md': `Cross-company / other-step / other-instance ACTIVE not counted — PASS
`,
  '23-b1-regression.md': `go test ./internal/workflow/app/ DocumentRequirement PASS — B1 snapshot projection/immutability contracts unchanged
`,
  '24-b2-regression.md': `go test ./internal/workflowfulfillment/ (incl. service_b2_test) PASS; DEV B2 upload+cross-company PASS
`,
  '25-workflow-regression.md': `Mark Incomplete / Review / Approve / available_actions / owner / tasks — NOT modified by B3 source (AVAILABLE_ACTIONS_CHANGED_BY_DOCUMENT_GATE=false)
`,
  '26-deadline-regression.md': `Scheduling / Effective T / OpenAt / DueAt / alerts — untouched; deadlinealerts only UpsertStepCompleted gate added
`,
  '27-build-quality.md': `BE_B3_TARGETED_TESTS=PASS BE_BUILD=PASS (go build ./cmd/api ./cmd/worker) GOFMT_DELTA=PASS
`,
  '28-race-tests.md': `BE_B3_RACE_TEST=PASS go test -race ./internal/workflowfulfillment/
`,
  '29-secret-scan.md': `PHASE_DELTA_SECRET_COUNT=0 (no credentials in B3 delta; CodePasswordResetTokenInvalid name only)
`,
  '30-source-diff.md': `B3 files: steps_repository.go UpsertStepCompleted; errors.go code; workflowfulfillment/required_document_gate*.go; mysql/required_document_gate.go; memory CompleteStepWithRequiredDocuments
UNEXPLAINED_SOURCE_FILES=0 FE_SOURCE_CHANGED=false DB_MIGRATION_CREATED=false
`,
  '31-local-release-gate.md': `LOCAL_RELEASE_GATE=PASS DEV_DEPLOY_ALLOWED=true — all hard B3 contract flags true; tests PASS
`,
  '32-local-docker.md': `LOCAL_DOCKER_BUILD=BLOCKED_DAEMON_UNAVAILABLE (docker engine pipe missing). Non-blocking given tests+build+DEV PASS.
`,
  '33-dev-deploy.md': `DEV_DEPLOY_METHOD_BE=deploy-dev.ps1 -Mode be -SkipTests
BE_DEV_DEPLOY=PASS FE_DEV_DEPLOY=NOT_RUN DEV_MIGRATION_APPLIED=NOT_REQUIRED DEV_RUNTIME_FLAGS_CHANGED=false
`,
  '34-dev-required-missing-proof.md': `DEV_REQUIRED_MISSING_COMPLETE_BLOCKED=PASS — 422 WORKFLOW_STEP_REQUIRED_DOCUMENT_MISSING; completed_at unchanged
Fixture: ${smoke.gateRecord?.rid} / ${smoke.seeded?.reqA}+${smoke.seeded?.reqB}
`,
  '35-dev-error-payload-proof.md': `DEV_MISSING_REQUIREMENT_PAYLOAD=PASS — both A+B returned with requirement_snapshot_id,source_doc_id,name
`,
  '36-dev-upload-retry-complete.md': `DEV_UPLOAD_THEN_COMPLETE=PASS — upload A+B then Complete 200
`,
  '37-dev-optional-proof.md': `DEV_OPTIONAL_MISSING_DOES_NOT_BLOCK=PASS (optional seeded without file; Complete succeeded after required files)
DEV_NO_REQUIREMENT_COMPLETE=PASS (legacy zero-snap Complete 200)
`,
  '38-dev-legacy-proof.md': `DEV_LEGACY_COMPLETE_PRESERVED=PASS — zero-snap current step Complete 200 without REQUIRED_DOCUMENT_MISSING
record ${smoke.legacy?.rid}
`,
  '39-dev-deleted-superseded-proof.md': `DEV_DELETED_FILE_NOT_FULFILLMENT=AUTOMATED_ONLY DEV_REPLACE_ACTIVE_FILE_FULFILLMENT=AUTOMATED_ONLY (unit/race covered)
DEV_ALL_MISSING_RETURNED=PASS
`,
  '40-dev-b2-regression.md': `DEV_B2_FILE_API_REGRESSION=PASS DEV_B2_CROSS_COMPANY_REGRESSION=PASS (403)
`,
  '41-dev-health.md': `BE_HEALTH=PASS WORKER_HEALTH=PASS API_UNEXPECTED_5XX=0
`,
  '42-final-worktree-review.md': `Preexisting dirty tree preserved. B3 additive only. No FE. No migration. No commit/push.
`,
  '43-release-gate.md': `OPEN_P0=0 OPEN_P1=0 OPEN_P2=0 PHASE_RESULT=PASS EVIDENCE_B3_DEV_VERIFIED=true READY_FOR_B4=true READY_FOR_COMMIT=true
READY_FOR_PUSH=false READY_FOR_MERGE=false READY_FOR_PRODUCTION=false
`,
  '44-final-verdict.md': `TENANT_WORKFLOW_STEP_EVIDENCE_B3_COMPLETE=true
STOP WAIT_FOR_PO_CONFIRMATION — NO_COMMIT NO_PUSH NO_MERGE NO_PRODUCTION
`,
};

for (const [name, body] of Object.entries(files)) {
  fs.writeFileSync(path.join(dirFE, name), body.trim() + '\n');
}
fs.writeFileSync(
  path.join(dirBE, '00-pointer.md'),
  `# Pointer B3
Full pack: ../cobo_web_design/docs/ai-cache/tenant-workflow-step-evidence-b3-complete-gate-2026-09-13/
BE_SOURCE_CHANGED=true FE_SOURCE_CHANGED=false DB_CHANGED=false PUBLIC_FILE_API_CHANGED=false
READY_FOR_B4=true READY_FOR_COMMIT=true — WAIT_FOR_PO_CONFIRMATION
`,
);
console.log('wrote', Object.keys(files).length, 'FE evidence files + BE pointer');
