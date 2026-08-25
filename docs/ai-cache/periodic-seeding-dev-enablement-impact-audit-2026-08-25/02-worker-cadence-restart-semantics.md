# 02 — Cadence + restart

```text
CONFIG_LOADED_AT_STARTUP=true
RUNTIME_HOT_RELOAD_SUPPORTED=false
WORKER_RESTART_REQUIRED_FOR_FLAG_CHANGE=true

WORKER_TICK_INTERVAL=5s  # DEV compose + printenv
WORKER_TICK_5S_EXPECTED_IN_DEV=true

WORKER_STARTUP_RUNS_IMMEDIATELY=false
FIRST_PERIODIC_RUN_AFTER_START=after first ticker fire (~5s); no seed before first tick
OTHER_JOBS_RUN_ON_RESTART=same ticker path (outbox, reminders) after first tick

SEED_EVERY_TICK=true  # when disclosureSvc != nil
MATERIALIZE_EVERY_TICK=true
INTERNAL_THROTTLE_OR_CADENCE=none beyond ListPendingCycles LIMIT 200
```

Call graph:

```text
.env → config.Load → cmd/worker main
→ if PeriodicSeedingEnabled: wire disclosureSvc + periodicCreator
→ time.NewTicker(WorkerTickInterval)
→ tick(): SeedPeriodicCycles → MaterializePeriodicDisclosures → reminders → outbox
```
