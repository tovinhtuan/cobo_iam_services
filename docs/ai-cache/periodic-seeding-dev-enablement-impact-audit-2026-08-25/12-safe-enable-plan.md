# 12 — Safe enable plan (DO NOT EXECUTE THIS AUDIT)

```text
STEP_1_PRECHECK=
  capture worker env PERIODIC_SEEDING_ENABLED WORKFLOW_SNAPSHOT_ENABLED
  SELECT COUNT(*) periodic_cycles, disclosure_records, workflow_instances
  confirm HCM date still 2026-08-25 if target recovery required
  confirm single cobo-iam-worker

STEP_2_CHANGE_CONFIG=
  Set in the config source that actually feeds the running worker
  (today: /root/cobo_project/.env AND/OR artifacts worker environment):
    PERIODIC_SEEDING_ENABLED=true
    WORKFLOW_SNAPSHOT_ENABLED=true
  Do not rely on override.yml unless deploy merges it.

STEP_3_RESTART=
  restart worker only (not full stack) after env change
  wait ≥1 tick (5s)

STEP_4_FIRST_TICK_VERIFY=
  logs: no panic; materialize errors for empty workflow only
  DB delta ≤ thresholds below

STEP_5_DATA_VERIFY=
  EXPECTED_NEW_CYCLES_FIRST_RUN≈52 (± applicability edge)
  EXPECTED_NEW_RECORDS_FIRST_RUN≤52
  EXPECTED_WORKFLOW_INSTANCES_FIRST_RUN≤52
  no duplicate (type,company,label)

STEP_6_TARGET_VERIFY=
  cycles for bang-tinh-luong-nhan-vien-thang-ban-sao / slot 2026-08-25 / 4 companies
  Draft records; Portal deadline alert for companies with access

STEP_7_OBSERVE_STEADY_STATE=
  N further ticks: cycle/record counts stable; expect noisy seeded logs

STEP_8_ACCEPT_OR_ROLLBACK=
  if deltas ≫ expected or orphan drafts without cycle.record_id → runtime rollback
```

Rollback triggers: new cycles>70; new records>70; orphans; panic; unexpected companies ≫4; 5xx spike.
