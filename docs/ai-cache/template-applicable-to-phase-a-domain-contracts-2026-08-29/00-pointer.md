# Template ApplicableTo Phase A — IAM pointer

```text
PHASE=A
APPLICATION_SOURCE_CHANGED=true
PRIMARY_EVIDENCE=../cobo_web_design/docs/ai-cache/template-applicable-to-phase-a-domain-contracts-2026-08-29/
```

## Diff (this repo)

- `internal/disclosure/app/contracts.go` — `ApplicableTo` on `TemplateDeadlineConfig`
- `internal/disclosure/app/applicable_to.go` — NEW
- `internal/disclosure/app/applicable_to_test.go` — NEW

## Verify

```text
go test ./internal/disclosure/app/ PASS
go build ./cmd/api ./cmd/worker PASS
BLOCKED: docker compose -f docker-compose.dev.yml build api — Docker daemon not running
```

```text
READY_FOR_PHASE_B=true
RUNTIME_APPLICABLE_TO_ENFORCEMENT_ACTIVE=false
NO_COMMIT / NO_PUSH / WAIT_FOR_CONFIRMATION
```
