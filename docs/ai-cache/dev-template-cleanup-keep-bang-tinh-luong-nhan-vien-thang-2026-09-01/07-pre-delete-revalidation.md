# 07 — Pre-delete revalidation

**Timestamp:** 2026-09-01 (execution phase)

## Environment

```text
ENVIRONMENT=DEV
PRODUCTION=false
DEV_HOST=88.216.208.0:21239 (hostname avi-server1)
DEV_DB=cobo_iam
DEV_API=http://88.216.208.0:8080
DEV_PATH=/root/cobo_project
```

## Inventory vs frozen audit

| Check | Result |
|-------|--------|
| PREDELETE_TEMPLATE_ROOT_COUNT | **42** |
| EXACT_KEEP_NAME_MATCH_COUNT | **1** |
| KEEP_ROOT_ID | `bang-tinh-luong-nhan-vien-ban-sao-2` |
| KEEP status / active_version_no | `active` / `1` |
| KEEP version_count | `1` |
| Frozen DELETE_ROOT_IDS count | **41** |
| KEEP in delete set | **0** |
| Missing frozen IDs in DB | **0** |
| Extra roots not in frozen∪KEEP | **0** |
| DELETE_SET_STATE_DRIFT | **false** |

## Candidate counts vs audit

All matched audit dry-run exactly: versions=106, cycles=195, records=195, wf_inst=184, wf_tasks=184, blocks=636, alert=30, dg=69, prefs=7, overrides=4, ov=19, gwv=4, non_draft=6, submitted=1.

## Business records approved for delete

| record_id | type_id | status | submitted |
|-----------|---------|--------|-----------|
| 01a0201e-120c-72ab-a30a-8a807e73a530 | bao-cao-tuan-test | PendingReview | 0 |
| 01a037b1-f577-76f0-8ea3-6d2202926375 | bao-cao-tuan-test | PendingReview | 1 |
| 01a01fc3-d04e-7136-9121-31d1fa44614c | qa-model-a-financial-20260820-1509 | PendingReview | 0 |
| 01a01fc3-d256-7c94-8307-bb698fa869d2 | qa-model-a-financial-20260820-1509 | PendingReview | 0 |
| 01a01fc3-d3f3-76b3-8bca-9a083d31da84 | qa-model-a-financial-20260820-1509 | PendingReview | 0 |
| 01a01fc8-518c-7108-90ee-edeaf44915df | qa-model-a-financial-20260820-1509 | PendingReview | 0 |

## Dependency graph

Live DB has almost no FK on template tables (only `workflow_task_assignees` → `workflow_tasks` CASCADE). Logical graph from audit + `type_id`/`record_id` column inventory unchanged. No new reference tables found.

```text
DEPENDENCY_GRAPH_COMPLETE=true
SHARED_MASTER_DATA_DELETE_RISK=false
KEEP_DATA_COLLATERAL_IMPACT=false
```
