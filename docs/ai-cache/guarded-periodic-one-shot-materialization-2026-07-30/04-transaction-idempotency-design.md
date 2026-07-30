# Transaction + idempotency

## Apply path

1. Validate env + allowlist
2. buildPlan (calculator + existing state)
3. Confirm token match
4. Freshness re-read (snapshot checksum)
5. Insert cycle if absent
6. Claim cycle
7. Materialize disclosure record via production path
8. On failure after cycle create: DeleteUnmaterializedCycle (rollback)
9. On both exist compatible: NO_OP_ALREADY_MATERIALIZED (mutations=0)

## Idempotency

- Second preview/apply → NO_OP_ALREADY_MATERIALIZED, no duplicate-key happy path
- DEV verified: counts remain cycle=1 record=1
