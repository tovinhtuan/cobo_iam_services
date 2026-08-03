# Task scope — Company Premium Backend Implementation Plan

**Mode:** SOURCE_AUDIT_AND_IMPLEMENTATION_PLAN_ONLY  
**Date:** 2026-08-03  

## In scope
- Deep Backend source audit (resolver, SQL, endpoints, authz, cache, tests)
- Domain separation: user tier vs company entitlement vs paid company plan
- API options + recommended contract (`RECOMMENDED_PENDING_APPROVAL`)
- Exact file/package impact map
- Phase-by-phase implementation plan, tests, migration, rollout/rollback, FE follow-up

## Out of scope
- Backend/Frontend source changes
- Migration execution
- DEV/Production deploy
- Push / force push
- Hardcoding Premium / using `user.subscriptionTier` as company plan
