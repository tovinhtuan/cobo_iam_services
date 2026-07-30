# Source discovery

## Call chain (production seeding)

```
worker periodic seed (PERIODIC_SEEDING_ENABLED)
→ disclosure app periodic materializer
→ template/version lookup + applicability
→ period builder (MONTHLY)
→ DeadlineCalculator.AddDurationInclusive (WORKING_DAYS)
→ InsertPeriodicCycle + ClaimCycle
→ MaterializeDisclosureRecord (workflow snapshot)
→ commit / link record_id
```

## Reused by one-shot

- Production `DeadlineCalculator` (exported `AddDurationInclusive`)
- MySQL repo: `GetPeriodicCycle`, `InsertPeriodicCycle`, `DeleteUnmaterializedPeriodicCycle`, claim/materialize paths
- Workflow snapshot materialize path (same as worker)
- Applicability checks via existing domain helpers

## Not used

- Global worker seed entrypoint (flag remains false)
- Direct SQL INSERT/UPDATE outside repository methods
