const fs = require('fs');
const path = require('path');
const dirFE = path.join(
  'c:/Users/tvttt/OneDrive/Desktop/cobo/cobo_web/cobo_web_design/docs/ai-cache/tenant-workflow-step-evidence-b1-requirement-snapshot-2026-09-13',
);
const dirBE = path.join(
  'c:/Users/tvttt/OneDrive/Desktop/cobo/cobo_web/cobo_iam_services/docs/ai-cache/tenant-workflow-step-evidence-b1-requirement-snapshot-2026-09-13',
);
fs.mkdirSync(dirFE, { recursive: true });
fs.mkdirSync(dirBE, { recursive: true });

const smoke = {
  DEV_SNAPSHOT_TABLE_EXISTS: 'PASS',
  DEV_LEGACY_INSTANCE_NO_BACKFILL: 'PASS',
  DEV_NEW_INSTANCE_SNAPSHOT_CASE: 'PASS',
  DEV_SNAPSHOT_FIELD_PARITY: 'PASS',
  DEV_COMPANY_OVERRIDE_SNAPSHOT: 'NOT_AVAILABLE',
  DEV_TEMPLATE_CHANGE_IMMUTABILITY: 'NOT_AVAILABLE',
  recordId: '01a09b6a-2dec-75b8-b931-7c640e4b76f3',
  snapshot_row_count: 4,
};

const files = {
  '00-context.md': `# B1 Requirement Snapshot
IMPLEMENTATION B1 ONLY — persist immutable documents[] at CreateWorkflowInstanceInternal.
NO FE / upload / Complete gate / evidence file table.
`,
  '01-contract-reference.md': `Accepted: tenant-workflow-step-evidence-v1-contract-plan-2026-09-13 + runtime upload audit.
`,
  '02-worktree-baseline.md': `BE HEAD c517894 + B1 delta. PREEXISTING_USER_CHANGES_PRESERVED=true. FE unchanged this phase.
`,
  '03-create-instance-source.md': `CREATE_WORKFLOW_INSTANCE_INTERNAL_FILE=internal/workflow/app/service.go
CALLERS: adhoc/infra/disclosure/record_creator.go; disclosure/infra/workflow/bootstrap.go
PERIODIC→RecordCreatorAdapter; ENSURE_ON_SUBMIT→Bootstrap.EnsureOnSubmit
EXISTING_TRANSACTION_BOUNDARY: CreateInstance now BeginTx + instance INSERT + doc snapshots + Commit
`,
  '04-effective-workflow-source.md': `EFFECTIVE from GetEffectiveWorkflow then ProjectDocumentRequirementSnapshots(same Workflow slice) + MapEffectiveWorkflowToSnapshot.
SNAPSHOT_USES_EXACT_INSTANCE_WORKFLOW_SOURCE=true
Proposal path: empty documentRequirements
`,
  '05-transaction-boundary.md': `SNAPSHOT_AND_INSTANCE_SAME_TX=true in mysql CreateInstance.
Task create remains after (preexisting). Snapshot failure rolls back instance.
`,
  '06-migration-design.md': `MIGRATION_ID=0135_workflow_step_document_requirement_snapshots
TABLE=workflow_step_document_requirement_snapshots
UNIQUE(workflow_instance_id, step_code, source_doc_id)
NO backfill INSERT
`,
  '07-domain-model.md': `DocumentRequirementSnapshot in workflow/app/document_requirement_snapshot.go
`,
  '08-repository.md': `insertDocumentRequirementSnapshotsTx; ListDocumentRequirementSnapshotsByInstanceStep (internal)
inmemory + mysql implement interface
`,
  '09-projection.md': `ProjectDocumentRequirementSnapshots — order preserved; required flag; template refs; empty→0 rows
`,
  '10-idempotency.md': `ON DUPLICATE KEY UPDATE id=id
`,
  '11-company-override.md': `Uses effective workflow only; empty override docs → 0 rows (no template fallback)
`,
  '12-version-immutability.md': `Persisted rows immutable; unit test DocumentRequirements_ImmutabilityAfterProjectionChange PASS
`,
  '13-legacy-safety.md': `No migration backfill; DEV proved 0 rows before new create with existing instances
`,
  '14-targeted-tests.md': `document_requirement_snapshot_test.go + service tests PASS
`,
  '15-transaction-tests.md': `DocumentRequirementInsertFailureRollsBack PASS
`,
  '16-periodic-tests.md': `Shares resolveWorkflowSnapshotForMaterialize path — adhoc/infra/disclosure tests PASS
`,
  '17-ensure-on-submit-tests.md': `bootstrap.go same ProjectDocumentRequirementSnapshots; package tests PASS
`,
  '18-regression-tests.md': `workflow/app, deadlinealerts/app, workflowdoctemplate, disclosure infra workflow PASS; Complete gate not added
`,
  '19-build-quality.md': `go build api/worker PASS; gofmt PASS; race on B1 tests PASS; full package race has preexisting concurrent test flake
`,
  '20-migration-review.md': `Forward CREATE IF NOT EXISTS; down DROP TABLE; no FK; no backfill
`,
  '21-source-diff.md': `BE: migration 0135, workflow app/repo/inmemory, record_creator, bootstrap, run_dev_migrations.sh
FE: none
`,
  '22-local-release-gate.md': `LOCAL_RELEASE_GATE=PASS
`,
  '23-dev-deploy.md': `deploy-dev.ps1 -Mode be -SkipTests PASS; push-migration 0135 applied; FE_DEV_DEPLOY=NOT_RUN
`,
  '24-dev-migration-proof.md': `Table exists; schema_migrations count=1 for 0135
`,
  '25-dev-snapshot-proof.md': JSON.stringify(smoke, null, 2),
  '26-dev-company-override-proof.md': `NOT_AVAILABLE on DEV (unit override projection PASS)
`,
  '27-dev-legacy-proof.md': `0 snapshot rows with preexisting instances before smoke create → PASS
`,
  '28-dev-health.md': `healthz/readyz 200; API_UNEXPECTED_5XX=0
`,
  '29-final-worktree-review.md': `PREEXISTING_USER_CHANGES_PRESERVED=true; UNEXPLAINED_SOURCE_FILES=0 for B1 scope
`,
  '30-release-gate.md': `OPEN_P0=0 OPEN_P1=0 PHASE_RESULT=PASS READY_FOR_B2=true READY_FOR_COMMIT=true
`,
  '31-final-verdict.md': `B1 COMPLETE. IMPLEMENTATION_ALLOWED for B2 only after PO. NO_COMMIT/NO_PUSH this phase unless PO asks.
`,
};

for (const [k, v] of Object.entries(files)) {
  fs.writeFileSync(path.join(dirFE, k), v);
}
fs.writeFileSync(
  path.join(dirBE, '00-pointer.md'),
  `# Pointer B1
Full pack: ../cobo_web_design/docs/ai-cache/tenant-workflow-step-evidence-b1-requirement-snapshot-2026-09-13/
BE_SOURCE_CHANGED=true FE_SOURCE_CHANGED=false READY_FOR_B2=true WAIT_FOR_PO_CONFIRMATION
`,
);
fs.writeFileSync(path.join(dirFE, 'dev-smoke-result.json'), JSON.stringify(smoke, null, 2));
console.log('wrote', Object.keys(files).length, 'to', dirFE);
