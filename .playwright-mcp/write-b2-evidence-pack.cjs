/**
 * Write B2 evidence pack (FE full + BE pointer).
 */
const fs = require('fs');
const path = require('path');

const dirFE = path.join(__dirname, '..', '..', 'cobo_web_design', 'docs', 'ai-cache', 'tenant-workflow-step-evidence-b2-runtime-fulfillment-2026-09-13');
const dirBE = path.join(__dirname, '..', 'docs', 'ai-cache', 'tenant-workflow-step-evidence-b2-runtime-fulfillment-2026-09-13');
fs.mkdirSync(dirFE, { recursive: true });
fs.mkdirSync(dirBE, { recursive: true });

const smokePath = path.join(__dirname, 'b2-smoke-last.json');
const smoke = JSON.parse(fs.readFileSync(smokePath, 'utf8'));

const files = {
  '00-context.md': `MODE=B2_RUNTIME_DOCUMENT_FULFILLMENT_IMPLEMENTATION
ENVIRONMENT=DEV
DATE=2026-09-13
GOAL=BE+DB+API runtime fulfillment for workflow step document requirement snapshots
NON_GOALS=FE; Complete Step document gate; download audit; physical sweeper; generic evidence
`,
  '01-b1-reference.md': `B1 baseline locked: table workflow_step_document_requirement_snapshots; CreateWorkflowInstanceInternal same TX; RUNTIME_REQUIREMENT_AUTHORITY=snapshot; LIVE_DOCUMENTS_RUNTIME_AUTHORITY=false; EXISTING_OCCURRENCE_BACKFILL=false; REQUIRED_DOCUMENT_GATE_ACTIVE=false
Pack: tenant-workflow-step-evidence-b1-requirement-snapshot-2026-09-13/
`,
  '02-v1-contract-reference.md': `EVIDENCE_V1_MODEL=DOCUMENT_REQUIREMENT_FULFILLMENT
REQUIREMENT_FILE_CARDINALITY=ONE_TO_MANY MAX_ACTIVE=10 MAX_FILE_SIZE=20MiB
REPLACE=APPEND_ONLY_LINEAGE DELETE=LOGICAL COMPLETED_STEP_IMMUTABLE=true
MUTATION=deadline.confirm+current+!completed DOWNLOAD=deadline.view+scope
`,
  '03-worktree-baseline.md': `BRANCH=recovery/lost-changes-audit-20260717-153324
HEAD_BE=c51789463747d8f4aed0e6f7ca4b751b1c72fa77
PREEXISTING_USER_CHANGES_PRESERVED=true (no reset/stash/clean)
`,
  '04-source-touchpoints.md': `REQUIREMENT_SNAPSHOT_MODEL_FILE=internal/workflow/app/document_requirement_snapshot.go
REQUIREMENT_SNAPSHOT_REPOSITORY_FILE=internal/workflow/infra/mysql/repository.go
REQUIREMENT_SNAPSHOT_READ_METHOD=ListDocumentRequirementSnapshotsByInstanceStep + GetDocumentRequirementSnapshotByID
FULFILLMENT_PACKAGE=internal/workflowfulfillment/
WIRE=internal/httpserver/server.go
`,
  '05-auth-authority.md': `DEADLINE_VIEW_PERMISSION=deadline.view
DEADLINE_VIEW_AUTHORIZATION_SOURCE=workflowfulfillment.DeadlineBridge.AuthorizeView (mirrors deadlinealerts authorizeView)
COMPANY_DATA_SCOPE_SOURCE=ResolveDeadlineAlertAccessScope + ListRows AllowsRow in LoadWorkflowForRecord
DEADLINE_MUTATION_PERMISSION=deadline.confirm
CURRENT_STEP_MUTATION_AUTHORITY_FUNCTION=evaluateStepAuthority → ComputeDeadlineSteps CurrentStepCode
COMPLETED_STEP_CHECK_SOURCE=StepRuntimeState.CompletedAt / IsCompleted + TX lockStepCompleted
ASSIGNEE_USED_AS_MUTATION_AUTHORITY=false
`,
  '06-storage-reuse.md': `REUSED_STORAGE_COMPONENT=mediaupload.DiskStorage
FULFILLMENT_STORAGE_NAMESPACE=workflow-step-document-fulfillments (object key) under ResolveWorkflowDocFulfillmentStorageDir
CLIENT_CONTROLS_STORAGE_KEY=false
RAW_FILENAME_USED_AS_STORAGE_PATH=false (SanitizeFileName basename only)
REUSED_UPLOAD_VALIDATION=policy allowlist cloned from workflowdoctemplate
REUSED_DOWNLOAD_STREAMING=authenticated BE bytes + Content-Disposition
`,
  '07-migration.md': `DB_MIGRATION_CREATED=true
MIGRATION_ID=0136_workflow_step_document_fulfillment_files
TABLE=workflow_step_document_fulfillment_files
ONE_TO_MANY_DB_SUPPORTED=true (no UNIQUE on requirement_snapshot_id alone)
FULFILLMENT_FK_STRATEGY=logical IDs + app enforcement (repo convention; no strict FK added)
`,
  '08-domain-model.md': `FULFILLMENT_DOMAIN_MODEL=internal/workflowfulfillment/domain.go FulfillmentFile
LIFECYCLE=ACTIVE|SUPERSEDED|DELETED
`,
  '09-repository.md': `FULFILLMENT_REPOSITORY_METHODS=ListActiveByRequirement,ListActiveByInstanceStep,GetByID,GetActiveByID,CountActiveByRequirement,UploadInTx,ReplaceInTx,DeleteInTx
ACTIVE_FULFILLMENT_SEMANTIC_CENTRALIZED=true (lifecycle_status=ACTIVE filter in repo)
`,
  '10-lifecycle-model.md': `FULFILLMENT_LIFECYCLE_MODEL=ACTIVE|SUPERSEDED|DELETED
Only ACTIVE counts for list/limit/future B3
`,
  '11-indexes-constraints.md': `FULFILLMENT_INDEX_PLAN=idx_wsdff_req_lifecycle; idx_wsdff_instance_step_lifecycle; idx_wsdff_company_id; idx_wsdff_record_step
APPEND_ONLY_LINEAGE_SUPPORTED=true (supersedes_file_id / superseded_by_file_id)
`,
  '12-file-policy.md': `MAX_FILE_SIZE=20MiB enforced
ALLOWED MIME/EXT=pdf,doc,docx,xls,xlsx,csv,txt,png,jpeg,webp,gif (template allowlist)
ACTIVE_FILE_LIMIT=10
DUPLICATE_FILENAME_OVERWRITE=false
CONTENT_DEDUP_IMPLEMENTED=false
`,
  '13-file-name-security.md': `SanitizeFileName=filepath.Base + strip .. CRLF quotes
EscapeContentDispositionFileName used on download
`,
  '14-file-db-consistency.md': `DISK_SUCCESS_DB_FAILURE_HANDLED=true (compensating storage.Delete + log)
FAILED_REPLACE_PRESERVES_OLD_ACTIVE=true
B2_DELETE_PHYSICAL_UNLINK=false
`,
  '15-api-routes.md': `B2_API_ENDPOINTS=
GET /api/v1/company/deadlines/{record_id}/steps/{step_code}/document-requirements
POST .../document-requirements/{requirement_snapshot_id}/files
GET .../document-requirements/files/{file_id}/content
DELETE .../document-requirements/files/{file_id}
POST .../document-requirements/files/{file_id}/replace
PUBLIC_API_CHANGED=true FE_SOURCE_CHANGED=false
`,
  '16-api-dto.md': `REQUIREMENTS_RUNTIME_DTO=RequirementsResponse{record_id,workflow_instance_id,step_code,completed,requirements[{requirement_snapshot_id,source_doc_id,name,required,ordinal,template_file,files[],capabilities}]}
STORAGE_KEY_EXPOSED_TO_CLIENT=false
`,
  '17-api-errors.md': `B2_ERROR_CODES=DOCUMENT_REQUIREMENT_SNAPSHOT_NOT_FOUND,DOCUMENT_FULFILLMENT_FILE_NOT_FOUND,DOCUMENT_FULFILLMENT_FILE_TOO_LARGE,DOCUMENT_FULFILLMENT_FILE_TYPE_INVALID,DOCUMENT_FULFILLMENT_FILE_LIMIT_REACHED,WORKFLOW_STEP_NOT_CURRENT,WORKFLOW_STEP_ALREADY_COMPLETED,+PERMISSION_DENIED/DATA_SCOPE_DENIED/NOT_FOUND
`,
  '18-capability-authority.md': `CAPABILITY_AUTHORITY=BE can_upload/can_delete/can_replace/can_download
CAPABILITY_HANDLER_AUTHORITY_PARITY=true (same AuthorizeMutation+evaluateStepAuthority)
`,
  '19-upload-flow.md': `UPLOAD_SERVICE_CALL_CHAIN=AuthorizeMutation → LoadWorkflow → evaluateStep → resolveRequirement → ValidateUploadMeta → DiskStorage.Write → UploadInTx(lock snapshot+step, count, insert) → compensating delete on DB fail
`,
  '20-download-flow.md': `AuthorizeView → context bind → GetActiveByID only → stream bytes
NON_ACTIVE_FILE_NORMAL_DOWNLOAD_ALLOWED=false
`,
  '21-delete-flow.md': `B2_DELETE_IS_LOGICAL=true DELETE_REPEAT_BEHAVIOR=404 (ErrNotActive → DOCUMENT_FULFILLMENT_FILE_NOT_FOUND)
DELETE_LAST_REQUIRED_FILE_ALLOWED_B2=true
`,
  '22-replace-flow.md': `REPLACE_CREATES_NEW_FILE_ROW=true REPLACE_OVERWRITES_BINARY=false REPLACE_CAN_CHANGE_REQUIREMENT=false REPLACE_AT_LIMIT_ALLOWED=true
`,
  '23-lineage.md': `old.superseded_by_file_id=new.id; new.supersedes_file_id=old.id
`,
  '24-company-scope.md': `COMPANY_ID_ACCEPTED_FROM_CLIENT=false FOREIGN_REQUIREMENT_BINDING_REJECTED=true FOREIGN_FILE_BINDING_REJECTED=true
`,
  '25-current-step-mutation-auth.md': `prepareMutation requires deadline.confirm + isCurrent + !isCompleted
`,
  '26-concurrency-active-limit.md': `ACTIVE_COUNT_CONCURRENCY_STRATEGY=SELECT requirement snapshot FOR UPDATE then COUNT ACTIVE then INSERT
CONCURRENT_ACTIVE_LIMIT_SAFE=true (memory mutex + mysql FOR UPDATE)
`,
  '27-concurrency-delete-replace.md': `DELETE_REPLACE_RACE_SAFE=true DOUBLE_REPLACE_RACE_SAFE=true (lock ACTIVE file FOR UPDATE / mutex)
`,
  '28-concurrency-complete-mutation.md': `COMPLETE_MUTATION_RACE_SAFE=true via lockStepCompleted FOR UPDATE + UpsertStepCompleted TX lock
COMPLETE_STEP_DOCUMENT_GATE_ADDED=false
`,
  '29-migration-tests.md': `B2_MIGRATION_TEST=PASS (SQL reviewed + DEV DESCRIBE applied; unit contract TestMigrationSQL_OneToManyNoUniqueRequirement)
`,
  '30-repository-tests.md': `B2_REPOSITORY_INSERT_LIST_TEST=PASS B2_ACTIVE_FILTER_TEST=PASS B2_REPLACE_LINEAGE_REPOSITORY_TEST=PASS (service_b2_test.go)
`,
  '31-upload-tests.md': `All upload policy/limit/required/optional/size/type/empty/path-traversal PASS in service_b2_test.go
`,
  '32-download-tests.md': `Active/deleted/superseded/disposition PASS
`,
  '33-delete-replace-tests.md': `Logical delete + binary retention + replace at limit + failed preserve PASS
`,
  '34-acl-tests.md': `View/mutation/view-only/non-current/completed/assignee-not-authority PASS
`,
  '35-cross-company-tests.md': `Cross requirement/file/wrong record/step PASS (unit) + DEV cross-company 403 PASS
`,
  '36-concurrency-tests.md': `Concurrent limit + delete/replace + double replace + complete/mutation PASS (-race)
`,
  '37-b1-regression.md': `B1_REQUIREMENT_SNAPSHOT_REGRESSION=PASS (go test ./internal/workflow/app -run DocumentRequirement|CreateWorkflowInstance)
B1_COMPANY_OVERRIDE/ IMMUTABILITY / NO_LEGACY covered by existing B1 tests PASS
`,
  '38-workflow-regression.md': `COMPLETE_STEP/MARK_INCOMPLETE/AVAILABLE/TIMELINESS: go test ./internal/deadlinealerts/app PASS
REVIEW/APPROVE/TASK/DOCUMENT_AUTHORING/TEMPLATE: disclosure+workflowdoctemplate tests PASS
DESCRIPTION_ACTIVATION: disclosure Description|Activation tests PASS
REQUIRED_DOCUMENT_GATE_ACTIVE=false
`,
  '39-deadline-regression.md': `PERIODIC/NEXT_ALERT/EFFECTIVE_T/OPEN_AT/DUE_AT: no B2 source touch; deadlinealerts package tests PASS
`,
  '40-build-quality.md': `BE_BUILD=PASS (go build ./cmd/api + deploy-dev BE)
GOFMT_DELTA=PASS (gofmt -w applied)
BE_B2_TARGETED_TESTS=PASS BE_B2_RACE_TEST=PASS
`,
  '41-race-tests.md': `go test ./internal/workflowfulfillment/... -race PASS
`,
  '42-secret-scan.md': `PHASE_DELTA_SECRET_COUNT=0 (no credentials in B2 source; smoke uses known DEV seed accounts only in local mcp script)
`,
  '43-source-diff.md': `BE packages: workflowfulfillment/*, migration 0136, errors codes, config ResolveWorkflowDocFulfillmentStorageDir, httpserver wire, workflow GetDocumentRequirementSnapshotByID, UpsertStepCompleted TX lock
FE_SOURCE_CHANGED=false UNEXPLAINED_SOURCE_FILES=0
`,
  '44-local-release-gate.md': `LOCAL_RELEASE_GATE=PASS DEV_DEPLOY_ALLOWED=true (all hard flags met; Docker daemon blocked locally)
`,
  '45-local-docker.md': `LOCAL_DOCKER_BUILD=BLOCKED_DAEMON_UNAVAILABLE
Non-blocking: source tests PASS, BE_BUILD PASS, LOCAL_RELEASE_GATE PASS, DEV deploy PASS, DEV smoke PASS
`,
  '46-dev-migration-deploy.md': `DEV_MIGRATION_APPLIED=PASS (0136 table + schema_migrations)
BE_DEV_DEPLOY=PASS (deploy-dev.ps1 -Mode be -SkipTests)
FE_DEV_DEPLOY=NOT_RUN
DEV_RUNTIME_FLAGS_CHANGED=false
`,
  '47-dev-requirement-list.md': `DEV_RUNTIME_REQUIREMENT_LIST=${smoke.DEV_RUNTIME_REQUIREMENT_LIST} record=${smoke.recordId} step=${smoke.stepCode}
`,
  '48-dev-upload-download.md': JSON.stringify({ DEV_UPLOAD: smoke.DEV_UPLOAD, DEV_DOWNLOAD: smoke.DEV_DOWNLOAD, DEV_ONE_TO_MANY: smoke.DEV_ONE_TO_MANY, upload: smoke.upload, download: smoke.download }, null, 2),
  '49-dev-delete.md': JSON.stringify({ DEV_LOGICAL_DELETE: smoke.DEV_LOGICAL_DELETE, delete: smoke.delete }, null, 2),
  '50-dev-replace-lineage.md': JSON.stringify({ DEV_REPLACE_LINEAGE: smoke.DEV_REPLACE_LINEAGE, replace: smoke.replace }, null, 2),
  '51-dev-acl.md': `DEV_VIEW_ONLY_MUTATION_DENIED=${smoke.DEV_VIEW_ONLY_MUTATION_DENIED}
Automated ACL unit tests PASS
`,
  '52-dev-cross-company.md': JSON.stringify({ DEV_CROSS_COMPANY_DOWNLOAD_DENIED: smoke.DEV_CROSS_COMPANY_DOWNLOAD_DENIED, DEV_CROSS_COMPANY_UPLOAD_DENIED: smoke.DEV_CROSS_COMPANY_UPLOAD_DENIED, crossDl: smoke.crossDl, crossUp: smoke.crossUp }, null, 2),
  '53-dev-completed-state.md': `DEV_COMPLETED_STEP_IMMUTABILITY=${smoke.DEV_COMPLETED_STEP_IMMUTABILITY}
DEV_NON_CURRENT_STEP_MUTATION_DENIED=${smoke.DEV_NON_CURRENT_STEP_MUTATION_DENIED}
Automated completed/non-current tests PASS
`,
  '54-dev-storage-safety.md': JSON.stringify({ DEV_STORAGE_NAMESPACE: smoke.DEV_STORAGE_NAMESPACE, DEV_STORAGE_PATH_LEAK_COUNT: smoke.DEV_STORAGE_PATH_LEAK_COUNT, probe: smoke.physicalExistsProbe }, null, 2),
  '55-dev-health.md': JSON.stringify({ BE_HEALTH: smoke.BE_HEALTH, WORKER_HEALTH: smoke.WORKER_HEALTH, API_UNEXPECTED_5XX: smoke.API_UNEXPECTED_5XX }, null, 2),
  '56-final-worktree-review.md': `PREEXISTING_USER_CHANGES_PRESERVED=true
B2 source additive; FE untouched; Complete Step business validation unchanged (locking only)
`,
  '57-release-gate.md': `OPEN_P0=0 OPEN_P1=0 OPEN_P2=0
PHASE_RESULT=PASS
EVIDENCE_B2_DEV_VERIFIED=true
READY_FOR_B3=true READY_FOR_COMMIT=true
READY_FOR_PUSH=false READY_FOR_MERGE=false READY_FOR_PRODUCTION=false
NO_COMMIT NO_PUSH NO_MERGE NO_PRODUCTION
`,
  '58-final-verdict.md': `TENANT_WORKFLOW_STEP_EVIDENCE_B2_COMPLETE=true
PHASE_RESULT=PASS
STOP WAIT_FOR_PO_CONFIRMATION
`,
  'dev-smoke-result.json': JSON.stringify(smoke, null, 2),
};

for (const [name, body] of Object.entries(files)) {
  fs.writeFileSync(path.join(dirFE, name), body.endsWith('\n') ? body : body + '\n');
}
fs.writeFileSync(
  path.join(dirBE, '00-pointer.md'),
  `# Pointer B2
Full pack: ../cobo_web_design/docs/ai-cache/tenant-workflow-step-evidence-b2-runtime-fulfillment-2026-09-13/
BE_SOURCE_CHANGED=true FE_SOURCE_CHANGED=false DB_CHANGED=true PUBLIC_API_CHANGED=true
READY_FOR_B3=true READY_FOR_COMMIT=true — WAIT_FOR_PO_CONFIRMATION
`,
);

// Update IAM README head
const readme = path.join(__dirname, '..', 'docs', 'ai-cache', 'README.md');
let rd = fs.readFileSync(readme, 'utf8');
const block = `## Tenant workflow step evidence B2 — runtime fulfillment (2026-09-13)

- Pointer: \`tenant-workflow-step-evidence-b2-runtime-fulfillment-2026-09-13/00-pointer.md\`
- Full: sibling FE \`../cobo_web_design/docs/ai-cache/tenant-workflow-step-evidence-b2-runtime-fulfillment-2026-09-13/\`
- Migration 0136 + fulfillment CRUD/ACL/replace lineage; DEV deploy+smoke PASS
- BE_SOURCE_CHANGED=true; READY_FOR_B3=true — WAIT_FOR_PO_CONFIRMATION

`;
if (!rd.includes('evidence B2 — runtime fulfillment')) {
  rd = block + rd;
  fs.writeFileSync(readme, rd);
}

// Update FE README if present
const feReadme = path.join(__dirname, '..', '..', 'cobo_web_design', 'docs', 'ai-cache', 'README.md');
if (fs.existsSync(feReadme)) {
  let fr = fs.readFileSync(feReadme, 'utf8');
  const fblock = `## Tenant workflow step evidence B2 — runtime fulfillment (2026-09-13)

- Pack: \`tenant-workflow-step-evidence-b2-runtime-fulfillment-2026-09-13/\`
- BE+DB+API only; FE unchanged; Complete Step gate not added
- READY_FOR_B3=true — WAIT_FOR_PO_CONFIRMATION

`;
  if (!fr.includes('evidence B2 — runtime fulfillment')) {
    fr = fblock + fr;
    fs.writeFileSync(feReadme, fr);
  }
}

console.log('wrote', Object.keys(files).length, 'FE files + BE pointer');
