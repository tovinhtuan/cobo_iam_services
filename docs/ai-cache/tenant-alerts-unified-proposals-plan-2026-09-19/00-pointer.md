# 00 — Pointer (IAM)

```text
PACK=tenant-alerts-unified-proposals-plan-2026-09-19
MODE=Phase 1 FE done — BE unchanged
DATE=2026-09-20
FULL_SOLUTION=sibling cobo_web_design/.../01-solution.md
FULL_IMPL_PLAN=sibling cobo_web_design/.../02-implementation-plan.md
```

## Status

```text
PLAN_ONLY=false
PHASE_1_IMPLEMENTED=true
BE_UNIFIED_FEED=DEFERRED
FE_PHASE_1=PARALLEL_EXISTING_APIS
CODE_MODIFIED=false (this repo)
MIGRATION_CREATED=false
API_CONTRACT_CHANGED=false
BE_CHANGE_REQUIRED_PHASE1=false
BE_CHANGE_REQUIRED_PHASE2=optional_alerts_feed
DEV_DEPLOYED=true (FE sibling)
STAGED=false
COMMITTED=false
READY_FOR_USER_COMMIT=true (FE sibling)
```

## BE facts locked from source

```text
BE_ALERT_LIST_ENDPOINT=GET /api/v1/company/deadline-alerts
BE_PROPOSAL_LIST_ENDPOINT=GET /api/v1/company/ad-hoc-proposals
PROPOSAL_RECORD_LINK_FIELD=record_id (+ workflow_instance_id); deterministic UUID SHA1(company:proposal)
EXISTING_UNIFIED_READ_MODEL=false
PROPOSAL_PERMISSION=ad_hoc_alert.propose|read|focal_review
ALERT_PERMISSION=deadline.view (list); deadline.confirm (confirm)
TENANT_SCOPE_ENFORCEMENT=JWT company_id on every query + list scopes (read vs scope=my)
```

## Recommendation

Phase 1: **no BE code** (done on FE). FE segmented unified screen.  
Phase 2: optional `GET .../alerts-feed` only if interleaved pagination required.
