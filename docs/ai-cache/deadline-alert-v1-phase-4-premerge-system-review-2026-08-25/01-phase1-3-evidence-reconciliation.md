# 01 — Phase 1–3 evidence reconciliation

| Phase | Evidence folder | Source match |
|-------|-----------------|--------------|
| 1 SQL membership | `deadline-alert-v1-phase-1-sql-membership-2026-08-25/` | PASS — `listRowsV1ObligationMembershipSQL` + `businessDateHCM` in HEAD |
| 2 Service | `deadline-alert-v1-phase-2-service-integration-2026-08-25/` | PASS — Draft skip removed; due/confirm preserved |
| 3 DEV | `deadline-alert-v1-phase-3-dev-verification-2026-08-25/` | PASS — evidence present; no re-deploy this phase |

```text
PHASE_1_SQL_MEMBERSHIP_RECONCILED=PASS
PHASE_2_SERVICE_INTEGRATION_RECONCILED=PASS
PHASE_3_DEV_EVIDENCE_RECONCILED=PASS
PHASE_3_DEV_GATE=PASS (prior)
```

```text
TEST_SETUP_ARTIFICIAL=QA CreateRecord + tagged periodic_cycles (qa-dav1-20260825)
BUT_RUNTIME_PATH_UNDER_TEST_REAL=ListRows SQL + ListDeadlineAlerts + SubmitRecord API + Portal /app/deadlines
```
