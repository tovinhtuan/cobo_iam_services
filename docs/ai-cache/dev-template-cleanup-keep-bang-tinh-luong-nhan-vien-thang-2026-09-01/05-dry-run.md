# Dry run — would-delete counts

**KEEP root:** `bang-tinh-luong-nhan-vien-ban-sao-2`  
**Method:** Read-only SQL (`_dependency_queries_v2.sql`, `_dependency_queries_v3.sql`, `_extra_dep_counts.sql`)

| Metric | Would delete |
|--------|-------------:|
| WOULD_DELETE_ROOTS | **41** |
| WOULD_DELETE_VERSIONS | **106** |
| WOULD_DELETE_PERIODIC_CYCLES | **195** |
| WOULD_DELETE_RECORDS | **195** |
| WOULD_DELETE_WORKFLOW_INSTANCES | **184** |
| WOULD_DELETE_WORKFLOW_TASKS | **184** |
| WOULD_DELETE_GLOBAL_WORKFLOWS | **0** |
| WOULD_DELETE_GLOBAL_STEPS | **0** |
| WOULD_DELETE_GLOBAL_WORKFLOW_VERSIONS | **4** |
| WOULD_DELETE_TEMPLATE_BLOCKS | **636** |
| WOULD_DELETE_DISPLAY_GROUPS | **69** |
| WOULD_DELETE_COMPANY_PREFS | **7** |
| WOULD_DELETE_WORKFLOW_OVERRIDES | **4** |
| WOULD_DELETE_OVERRIDE_VERSIONS | **19** |
| WOULD_DELETE_ALERT_TEMPLATE_CONFIGS | **30** |
| WOULD_DELETE_AD_HOC_PROPOSALS | **0** |
| WOULD_DELETE_WORKFLOW_OVERRIDE_CONFLICTS | **0** |
| WOULD_DELETE_DEADLINE_CONFIRMATIONS | **0** |
| WOULD_DELETE_FILES | **0** (DB-scoped; no blob delete audited) |

```text
WOULD_KEEP_ROOTS=1
```

## P0 — business records on delete candidates

| Metric | Count |
|--------|------:|
| Non-draft records | **6** |
| Submitted records (`submitted_at IS NOT NULL`) | **1** |

```text
DELETE_CANDIDATE_HAS_EXISTING_BUSINESS_RECORDS=true
```

Deleting candidates will **destroy historical disclosure/workflow data** for those templates (by design if confirmed).

## KEEP template unaffected

All counts above exclude KEEP `type_id`. KEEP baseline documented in `03-keep-root-resolution.md`.
