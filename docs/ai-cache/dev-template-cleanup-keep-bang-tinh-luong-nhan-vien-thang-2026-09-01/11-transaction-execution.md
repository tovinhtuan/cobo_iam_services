# 11 — Transaction execution

## Method

```text
DELETE_EXECUTION_METHOD=TRANSACTIONAL_SQL
DELETE_ATOMICITY=FULL_DB_TRANSACTION
DELETE_SET_SOURCE=AUDITED_FROZEN_ROOT_IDS
```

Single MySQL session via stored procedure `cobo_dev_template_cleanup_tx` (created, called, dropped).

## Safety flags

```text
FOREIGN_KEY_CHECKS_DISABLED=false
TRUNCATE_USED=false
NAME_BASED_MASS_DELETE_USED=false
ARCHIVE_USED_AS_DELETE=false
KEEP_ROOT_RECREATED=false
```

## Delete order (child-first)

1. workflow_tasks (via records of frozen roots) — assignees CASCADE
2. workflow_instances
3. deadline_alert_confirmations
4. disclosure_records
5. periodic_cycles
6. company_template_workflow_override_versions
7. company_template_workflow_overrides
8. company_type_preferences
9. alert_template_configs
10. disclosure_template_blocks
11. global_workflow_steps
12. global_workflow_versions
13. global_workflows
14. template_display_groups
15. ad_hoc_proposals
16. workflow_override_conflicts
17. disclosure_type_versions
18. disclosure_types

## Affected rows

| Table / entity | Deleted |
|----------------|--------:|
| roots | 41 |
| versions | 106 |
| cycles | 195 |
| records | 195 |
| workflow_instances | 184 |
| workflow_tasks | 184 |
| template_blocks | 636 |
| alert_template_configs | 30 |
| display_groups | 69 |
| company_prefs | 7 |
| overrides | 4 |
| override_versions | 19 |
| global_workflow_versions | 4 |
| global_workflows / steps / dac / adhoc / woc | 0 |

```text
DELETE_TRANSACTION_STARTED=true
```
