# Template ApplicableTo Phase C — IAM pointer

```text
PHASE=C
PRIMARY_EVIDENCE=../cobo_web_design/docs/ai-cache/template-applicable-to-phase-c-worker-upper-bound-2026-08-29/
```

## Diff

- `periodic.go` — EvaluateApplicableToEligibility before UpsertPeriodicCycle
- `contracts.go` — PeriodicTypeRow.ApplicableTo
- `mysql/repository.go` — JSON extract applicable_to
- `periodic_applicable_to_seed_test.go` — NEW
- `repository_periodic_test.go` — SQL assert

```text
go test ./internal/disclosure/app/ PASS
go build ./cmd/api ./cmd/worker PASS
LOCAL_DOCKER_BUILD=NOT_RUN_DAEMON_UNAVAILABLE
READY_FOR_NEXT_PHASE=true
DEV_DEPLOY_PERFORMED=false
```
