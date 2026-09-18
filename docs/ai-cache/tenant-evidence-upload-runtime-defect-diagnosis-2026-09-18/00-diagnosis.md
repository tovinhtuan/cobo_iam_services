# Tenant evidence upload runtime defect — diagnosis (2026-09-18)

MODE=TARGETED_DEFECT_DIAGNOSIS
SOURCE_CHANGED=false

## Classification
CASE=CASE_GET_EMPTY
ROOT_CAUSE=OTHER_PROVEN_ROOT_CAUSE
SUBTYPE=AUTHORING_EMPTY_DOCUMENTS_ZERO_SNAPSHOT
INSTANCE_CLASS=NEW_INSTANCE_POST_B1 (created 2026-09-17; mig 0135 present since ~2026-09-13)
NOT=LEGACY_PRE_B1
NOT=B1_MATERIALIZATION_CODE_DEFECT (projection+insert correct when documents[] non-empty)

## Primary fixture (sanitized)
- company_id=c_001
- record_id=01a0b04f-a46c-71ad-ae9d-a23389ef3c21
- workflow_instance_id=01a0b04f-a470-7b8a-b044-d916c53f963e
- type_id=qa-final-focused-20260904222350
- step_code=step-001
- workflow_source=global_template
- created_at=2026-09-17 17:00:04

## Runtime
- GET .../document-requirements → 200 requirements=[]
- SNAPSHOT_ROW_COUNT=0
- completed=false
- UPLOAD_CTA_VISIBLE=false (empty panel; no DocumentRequirementCard)
- UPLOAD_REQUEST_SENT=false
- Survey: 12/12 incomplete deadline alerts for c_001 → requirements=[]

## Authoring proof
- disclosure_type_versions.workflow_manifest_json for all current incomplete QA types: steps[*].documents = []
- Counter-proof B1 works: type bang-tinh-luong-nhan-vien-thang has 4 docs; WIs 2026-09-13 materialized 4 snaps at create time

## Why B4/B6 PASS did not catch this
- Smoke scripts INSERT snapshots directly (b2/b3/b5/b6-*-smoke.cjs), bypassing authoring→CreateWorkflowInstanceInternal projection

## Authz (not blocking upload CTA)
- Tenant has deadline.view + deadline.manage (deadline.confirm aliased to manage)
- STEP_COMPLETED=false; current step step-001

## Minimal fix scope (no implement this phase)
1. Author non-empty documents[] on CMS/company effective workflow for types tenant uses
2. New materializations will get B1 snaps
3. Existing zero-snap WIs: V1 says no backfill — need PO decision (recreate vs approved backfill)
4. Harden release smoke to assert real materialization path (no SQL seed as sole B1 proof)

## Worktree
- FE HEAD 6f21000 recovery/lost-changes-audit-20260717-153324 clean
- BE HEAD 4639c5d same branch clean
