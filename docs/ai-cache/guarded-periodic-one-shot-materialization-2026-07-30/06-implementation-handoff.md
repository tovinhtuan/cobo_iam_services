# Implementation handoff

## Packages

- `internal/disclosure/app/periodic_oneshot/` — engine, allowlist, token, env, production/memory domain
- `cmd/periodic-materialize-one` — thin CLI
- Repo: Get/Insert/DeleteUnmaterialized periodic cycle
- `DeadlineCalculator.AddDurationInclusive` exported for production calc reuse

## DEV artifact

- Binary SCP: `/root/cobo_project/bin/periodic-materialize-one`
- No `make deploy-be` / API recreate (CLI-only isolation)
