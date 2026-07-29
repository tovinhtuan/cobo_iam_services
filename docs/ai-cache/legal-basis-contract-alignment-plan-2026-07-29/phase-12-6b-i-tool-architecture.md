# Tool architecture

- cmd/legal-basis-backfill (plan default; apply memory-gated)
- cmd/legal-basis-verify
- cmd/legal-basis-rollback
- package legal_basis_backfill: allowlist, env, snapshot, transform, MemoryStore, executor, verify, rollback
