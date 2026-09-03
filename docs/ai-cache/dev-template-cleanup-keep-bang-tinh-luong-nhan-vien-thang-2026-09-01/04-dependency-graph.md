# Dependency graph — DEV schema

## Tables with `type_id` column

| Table | FK to `disclosure_types` | Notes |
|-------|--------------------------|-------|
| `disclosure_type_versions` | Yes (RESTRICT) | Child versions |
| `disclosure_template_blocks` | via `(type_id, version_no)` → versions | Block JSON incl. workflow |
| `template_display_groups` | Yes (**ON DELETE CASCADE**) | Junction |
| `periodic_cycles` | Yes (RESTRICT) | Seeded cycles |
| `disclosure_records` | **No FK** (column only) | Linked by `type_id` |
| `company_type_preferences` | Yes (RESTRICT) | Per-company prefs |
| `company_template_workflow_overrides` | Yes (RESTRICT) | Override header |
| `company_template_workflow_override_versions` | CASCADE from overrides | |
| `global_workflows` | Yes (RESTRICT) | Often empty; workflow in blocks |
| `global_workflow_steps` | via `workflow_id` CASCADE | |
| `global_workflow_versions` | No FK in migrations | `type_id` column |
| `alert_template_configs` | **No FK** | Email alert mapping |
| `workflow_override_conflicts` | Unknown/manual | |
| `ad_hoc_proposals` | Unknown/manual | |

## Record-linked tables (via `record_id`)

| Table | Links |
|-------|-------|
| `workflow_instances` | `record_id` → `disclosure_records` |
| `workflow_tasks` | `workflow_instance_id` |
| `deadline_alert_confirmations` | `record_id` |
| `periodic_cycles.record_id` | nullable until materialized |

## Dependency tree (conceptual)

```text
disclosure_types (root)
├── disclosure_type_versions
│   └── disclosure_template_blocks
├── template_display_groups (CASCADE on root delete)
├── periodic_cycles
│   └── disclosure_records (via record_id)
├── disclosure_records (type_id, no FK)
│   ├── workflow_instances
│   │   └── workflow_tasks
│   └── deadline_alert_confirmations
├── company_type_preferences
├── company_template_workflow_overrides
│   └── company_template_workflow_override_versions (CASCADE)
├── global_workflows → global_workflow_steps (CASCADE)
├── global_workflow_versions
├── alert_template_configs (no FK)
└── ad_hoc_proposals / workflow_override_conflicts
```

## FK audit summary

Most child tables use **RESTRICT** (default) on `disclosure_types` — root cannot be deleted until children removed manually.

Exception: `template_display_groups` **CASCADE** on root delete.

`disclosure_type_versions` blocks root delete until versions deleted.

```text
FK_AUDIT_COMPLETE=true
DEPENDENCY_GRAPH_COMPLETE=true
```

## File storage

Workflow/document metadata in `disclosure_template_blocks` / `global_workflow_steps.documents_json`. No audited canonical object-storage delete path tied to template root delete.

```text
TEMPLATE_FILE_CLEANUP_REQUIRED=unknown (DB metadata only in scope)
PHYSICAL_FILE_DELETE_SAFE=false (do not delete blobs without separate audit)
AUDIT_HISTORY_DELETED=false (preference: retain audit tables)
```
