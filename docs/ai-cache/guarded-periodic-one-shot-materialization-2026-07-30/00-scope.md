# Scope — Guarded Periodic One-shot Materialization

- Phase: Guarded Periodic One-shot Materialization
- Environment: DEV only (`avi-server1` / 88.216.208.0)
- Exact allowlist:
  - type_id: `qa-monthly-deadline-alert-202607-1785382733`
  - company_id: `c_001`
  - period: `2026-07`
- Forbidden: PERIODIC_SEEDING_ENABLED flip, wildcard, migration, direct SQL writes, production, MySQL recreate, FE changes
- Delivery: guarded CLI `cmd/periodic-materialize-one` + `internal/disclosure/app/periodic_oneshot`
