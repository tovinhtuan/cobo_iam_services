# 05 — Phase 1 regression

```bash
go test ./internal/deadlinealerts/infra/mysql/... -count=1
```

```text
PHASE_1_REPOSITORY_REGRESSION=PASS
PHASE_1_SQL_MEMBERSHIP_CHANGED_BY_PHASE_2=false
git diff -- internal/deadlinealerts/infra/mysql/ → empty in Phase 2 worktree delta
```

Preserved Phase 1 contract flags (via mysql suite):

```text
PERIODIC_PRE_OPENAT_REPOSITORY_EXCLUSION=PASS
PERIODIC_OPENAT_REPOSITORY_INCLUSION=PASS
POST_SUBMIT_REPOSITORY_EXCLUSION=PASS
IRREGULAR_REPOSITORY_BEHAVIOR=PASS
HCM_BUSINESS_DATE=PASS
```
