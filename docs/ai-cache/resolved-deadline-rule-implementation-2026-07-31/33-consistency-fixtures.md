# Consistency fixtures

Canonical machine-readable fixtures:

`src/pages/portal/disclosure-detail/__fixtures__/resolvedDeadlineConsistency.json`

Consumed by:
- FE: `resolvedDeadlineConsistency.crossLayer.test.tsx`
- BE: mirrored expectations in `applicability/phase4_consistency_matrix_test.go` (same Case IDs C1–C9)

Each case documents: CMS/profile input → backend expected → `apiJson` → FE sentence/context.
