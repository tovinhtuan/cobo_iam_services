# Delete plan (NOT EXECUTED)

## Blocker — canonical hard delete

User requested **xóa** (hard delete). Application only exposes **archive** (soft hide).

```text
IMPLEMENTATION_BLOCKED=true
BLOCKER=NO_CANONICAL_HARD_DELETE_PATH
```

Archive would leave 41 rows in `disclosure_types` — **does not meet** user goal of only 1 template root.

## Proposed execution method (after explicit confirmation)

```text
DELETE_EXECUTION_METHOD=TRANSACTIONAL_SQL
DELETE_ATOMICITY=FULL_DB_TRANSACTION (DB only; worker race remains)
```

**Not** `DELETE FROM disclosure_types WHERE name <> ...` — use frozen `DELETE_ROOT_IDS` list.

### Pre-execution checklist

1. Re-run inventory + KEEP resolution (state drift guard)
2. Optional: `mysqldump cobo_iam` on DEV host
3. Consider pausing `cobo-iam-worker` briefly (out of scope unless user approves)
4. `START TRANSACTION`
5. Delete children **for DELETE_ROOT_IDS only** (child-first), e.g.:
   - `workflow_tasks` / `workflow_instances` for records where `type_id IN (...)` 
   - `deadline_alert_confirmations` for those records
   - `disclosure_records` where `type_id IN (...)`
   - `periodic_cycles` where `type_id IN (...)`
   - `company_template_workflow_override_versions` via overrides
   - `company_template_workflow_overrides` where `type_id IN (...)`
   - `company_type_preferences` where `type_id IN (...)`
   - `alert_template_configs` where `type_id IN (...)` (no FK)
   - `disclosure_template_blocks` where `type_id IN (...)`
   - `global_workflow_versions` / `global_workflows` / steps where applicable
   - `disclosure_type_versions` where `type_id IN (...)`
   - `disclosure_types` where `type_id IN (...)`
6. Verify `COUNT(*) FROM disclosure_types = 1` and name exact match
7. Verify KEEP baseline unchanged
8. `COMMIT` or `ROLLBACK` on any failure

### Explicitly NOT in plan

- `TRUNCATE disclosure_types`
- `SET FOREIGN_KEY_CHECKS=0`
- Name-based `WHERE name <> ...`
- Production / source code changes
- Physical file/blob deletion without separate audit

## Alternative (does NOT satisfy user goal)

Archive all 41 via `POST .../archive` — leaves DB rows; Portal hides most. **Rejected** for stated goal.

## Confirmation summary

```text
KEEP:
bang-tinh-luong-nhan-vien-ban-sao-2 — Bảng tính lương nhân viên tháng

DELETE:
41 template roots

Dependent rows (approx):
versions=106, cycles=195, records=195, workflow_instances=184, workflow_tasks=184,
template_blocks=636, display_groups=69, alert_configs=30, overrides=4, ...

Execution:
Transactional scoped SQL (no canonical API)

Collateral impact:
6 non-draft + 1 submitted record on delete candidates; KEEP data preserved

Rollback:
Manual mysqldump restore (recommend snapshot before execute)
```

```text
DELETE_EXECUTED=false
```
