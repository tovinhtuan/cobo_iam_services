# 08 — Root cause

## Classification

```text
ROOT_CAUSE_CLASS=I_PERIODIC_SEEDING_DISABLED
FIRST_BROKEN_BOUNDARY=CHECK_05 / PERIODIC_SEEDING_ENABLED (worker enablement)
ROOT_CAUSE_LAYER=CONFIG
```

## Causal chain

```text
ROOT_CAUSE=
Template ACTIVE at T0≈2026-08-25 16:13 HCM
(DAILY, ApplicableFrom CURRENT_SLOT frozen to 2026-08-25, eligible for current slot)
→ DEV PERIODIC_SEEDING_ENABLED=false
→ cmd/worker never constructs disclosureSvc / never calls SeedPeriodicCycles or MaterializePeriodicDisclosures
→ no periodic_cycle for 2026-08-25
→ no disclosure_record
→ Deadline Alert membership correctly empty
→ Portal "Cảnh báo về thời hạn" shows nothing for this template
```

## Evidence

```text
SOURCE_EVIDENCE=
cmd/worker/main.go gate cfg.PeriodicSeedingEnabled;
config default false;
ResolveLogicalSlot DAILY supported

DEV_DATA_EVIDENCE=
.env + worker/api printenv PERIODIC_SEEDING_ENABLED=false;
cycles=0; records=0; active_version_no=1; af_slot=2026-08-25
```

## Distinctions

```text
Template ACTIVE ≠ occurrence exists ≠ Deadline Alert eligible
periodic_cycle exists ≠ disclosure_record materialized
Draft record ≠ alert before OpenAt
```

Here: ACTIVE true; occurrence false; alert N/A.

## Mid-day / recovery (if seeding were enabled)

```text
ACTIVATED_AFTER_LAST_SEED_RUN=N/A (seed never runs)
CAN_NEXT_SEED_RUN_CREATE_CURRENT_DAY_SLOT=true IF flag flipped and worker restarted/reloaded with true
  (tick 5s; current slot still 2026-08-25 until day rollover)
CURRENT_DAY_OCCURRENCE_PERMANENTLY_MISSED=false under enabled+ticking seeder
EXPECTED_TO_SELF_RECOVER=false while flag remains false
SEVERITY=P1 (DEV config / ops; not Deadline Alert SQL defect)
FIX_REQUIRED=true (ops/config — not application code in this audit)
RECOMMENDED_FIX_LAYER=CONFIG
IMPLEMENTATION_STARTED=false
```

No solution design beyond layer.