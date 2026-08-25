# 01 — Flag → runtime wiring

```text
FLAG_CONFIG_KEY=PERIODIC_SEEDING_ENABLED
CONFIG_STRUCT_FIELD=Config.PeriodicSeedingEnabled
CONFIG_SOURCE=boolEnv("PERIODIC_SEEDING_ENABLED", false) // internal/platform/config/config.go
CONFIG_PRECEDENCE=
  1) process environment at worker startup (immutable)
  2) compose `environment:` > `env_file: .env` when that compose file is used
  3) code default false

DEV_CONFIG_FILE_VALUE=false  # /root/cobo_project/.env
DEV_COMPOSE_VALUE=
  artifacts worker: not set (inherits .env false)
  override.yml worker: 'true' (often NOT applied by deploy-be artifacts-only)
  docker-compose.dev.yml: "true" (local parity; not current DEV runtime)
DEV_RUNNING_WORKER_ENV_VALUE=false
DEV_RUNNING_API_ENV_VALUE=false
CONFIG_VALUES_CONSISTENT=false
  # .env=false; override wants true; running=false

WORKER_GATE_FILE=cmd/worker/main.go
WORKER_GATE_SYMBOL=cfg.PeriodicSeedingEnabled (startup if)
FLAG_CHECK_TIME=startup
FLAG_SCOPE=seed_and_materialize
```

When false: `disclosureSvc` and `periodicCreator` stay nil → Seed + Materialize not called.

```text
FLAG_CONTROLS_SEED=true
FLAG_CONTROLS_MATERIALIZE=true
FLAG_CONTROLS_OTHER_JOBS=none
NON_PERIODIC_WORKER_JOBS_AFFECTED=false
BLAST_RADIUS_INCREASED=false  # for unrelated jobs
```

Unrelated jobs still run every tick: outbox reaper/processor, reminder seed/materialize/dispatch (gated by ReminderDispatchEnabled).
