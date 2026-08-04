# Phase 5 — PatchOwnCompany runtime safety

## Observed (DEV)

Account: `admin.dn@example.com` on `c_001`.

1. GET company → `plan` Premium ACTIVE.
2. PATCH `{phone:"P5SMOKE"}` → 200; phone updated; **plan unchanged Premium**.
3. PATCH `{phone:""}` restore → 200; phone cleared; plan still Premium.

No second reader failure after commit observed. Pre-mutation plan resolve (Phase 4 fix) confirmed by successful response with plan after mutation.

STRICT failure-before-mutation path: covered by Phase 4 unit tests (`TestPatchOwnCompany_PlanErrorBeforeMutation_NoUpdate`); not fault-injected on shared DEV.
