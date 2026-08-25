# 14 — GO / NO-GO

```text
ENABLEMENT_VERDICT=CONDITIONAL_GO
SAFE_TO_ENABLE_PERIODIC_SEEDING_ON_DEV=false
UNTIL_CONDITIONS_ACKNOWLEDGED=true

OPEN_P0=1
  WORKFLOW_SNAPSHOT_ENABLED unset on running worker → orphan Draft risk if PERIODIC enabled alone
OPEN_P1=3
  immediate ~24 OVERDUE alerts (late current-slot catch-up)
  5s NOOP upsert + log spam
  target 2026-08-25 slot not recoverable after HCM rollover without explicit recovery path
OPEN_P2=1
  seeded counter ≠ new-cycle delta (verification footgun)
```

Mandatory conditions:

1. Enable **both** `PERIODIC_SEEDING_ENABLED=true` and `WORKFLOW_SNAPSHOT_ENABLED=true` on the **worker** process (not API-only).
2. Acknowledge blast radius: ~52 cycles, ≤52 Draft records+workflows, ~28 alerts (incl. ~24 OVERDUE) across **4** companies.
3. Prefer enable while HCM date is still **2026-08-25** for target DAILY recovery; after rollover `2026-08-25` is permanently missed by current-slot seeder.
4. Capture pre-counts; worker-only restart; verify DB deltas ≤ thresholds; observe steady state.
5. Accept CONFIG rollback does not undo data.
6. No production; DEV only.

Naive “flip PERIODIC only” = **NO_GO**.
