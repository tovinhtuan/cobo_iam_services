# 04 — Test results

```bash
go test ./internal/deadlinealerts/infra/mysql/... -count=1
```

```text
PASS  TestListRows_usesActiveTemplateInnerJoin
PASS  TestBusinessDateHCM_fixedClock
PASS  TestListRows_usesV1ObligationMembershipSQL
PASS  TestListRowsV1Membership_acceptanceMatrix (all subtests)
PASS  TestListRows_preservesCompanyOrderScopeWiring
PASS  TestWithNow_overridesBusinessDateArgPath
ok    github.com/cobo/cobo_iam_services/internal/deadlinealerts/infra/mysql
EXIT_CODE=0
```

```text
REPOSITORY_TESTS=PASS
TESTS_USE_DETERMINISTIC_BUSINESS_DATE=PASS
SERVICE_TESTS_AS_FEATURE_PROOF=not_used
DEV_E2E=not_run (intentional)
```
