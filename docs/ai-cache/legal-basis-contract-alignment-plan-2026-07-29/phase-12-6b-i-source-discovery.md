# Phase 12.6B-I — Source discovery

- Branch: recovery/lost-changes-audit-20260717-153324
- Ancestors OK: 0c6dcca, 8dc045b, a07fd4d; safety 2d9a5b7
- Go: go1.24.9
- Reuse: ProjectLegalBasesToLegacy, ValidateLegalBasesForWrite, UUIDv7Generator, inventory Classify
- New package: internal/disclosure/app/legal_basis_backfill
- New cmds: legal-basis-backfill, legal-basis-rollback, legal-basis-verify
- DEV SQL write wiring: not enabled in 12.6B-I
