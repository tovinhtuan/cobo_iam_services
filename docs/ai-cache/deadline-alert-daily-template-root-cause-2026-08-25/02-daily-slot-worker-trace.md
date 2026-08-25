# 02 — Daily slot + worker call graph (source)

## Config gate (first broken operational boundary)

```text
PERIODIC_SEEDING_ENABLED=false
```

Sources:

- `/root/cobo_project/.env` on DEV host
- `docker exec cobo-iam-worker printenv PERIODIC_SEEDING_ENABLED` → `false`
- `docker exec cobo-iam-api printenv PERIODIC_SEEDING_ENABLED` → `false`
- Default in code: `boolEnv("PERIODIC_SEEDING_ENABLED", false)` (`internal/platform/config/config.go`)

## Worker wiring

```text
cmd/worker/main.go
  if sqlDB != nil && cfg.PeriodicSeedingEnabled {
    disclosureSvc = NewService(...)
    periodicCreator = ...
  }
  // else disclosureSvc remains nil
```

Tick (`WorkerTickInterval`, DEV=`5s` via env / default 5s):

```text
Worker trigger (time.Ticker cfg.WorkerTickInterval)
→ SeedPeriodicCycles (skipped when disclosureSvc == nil)
→ MaterializePeriodicDisclosures (skipped when disclosureSvc == nil)
```

## Seed path (would run if enabled)

```text
SeedPeriodicCycles
→ internal/disclosure/app/periodic.go seedPeriodicCycles
→ ResolveLogicalSlot(frequency, now, Asia/Ho_Chi_Minh)  // effective_schedule.go
   DAILY → n.Format("2006-01-02")  // YYYY-MM-DD
→ EvaluateApplicableFromEligibility(candidate, applicable_from_slot)
→ cycle upsert (periodic_cycles)
```

```text
DAILY_WORKER_BRANCH=ResolveLogicalSlot case PeriodicityDaily
DAILY_SLOT_FORMAT=2006-01-02 (YYYY-MM-DD)
DAILY_BRANCH_SUPPORTED=true
RAW_CANDIDATE_GENERATION=current logical slot only (no historical range)
CURRENT_CANDIDATE_SLOT=2026-08-25
BOUNDARY_SLOT=2026-08-25
COMPARE_RESULT=candidate >= boundary → eligible
CHECK_08_APPLICABLE_FROM_ELIGIBLE=PASS (would pass if seed ran)
```

## Mid-day activation support (conditional)

```text
DAILY_CURRENT_SLOT_MIDDAY_ACTIVATION_SUPPORTED=true
```

**If** `PERIODIC_SEEDING_ENABLED=true`: next tick (≤5s on DEV) re-resolves current daily slot and would upsert today's cycle. Late same-day activation is **not** permanently missed by cadence alone.

**With** seeding disabled: no tick creates cycles regardless of mid-day vs morning.

## Seed run after activate

```text
SEED_WORKER_TRIGGER=time.NewTicker(cfg.WorkerTickInterval)
SEED_WORKER_INTERVAL_OR_CRON=5s (DEV printenv WORKER_TICK_INTERVAL)
FIRST_SEED_RUN_AFTER_ACTIVATION=N/A — SeedPeriodicCycles never invoked (disclosureSvc nil)
CHECK_06_WORKER_RAN_AFTER_ACTIVATE=FAIL
```

Worker **process** ticks continuously; **periodic seed path** does not run.