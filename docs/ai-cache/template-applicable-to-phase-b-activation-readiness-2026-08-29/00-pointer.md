# Template ApplicableTo Phase B — IAM pointer

```text
PHASE=B
PRIMARY_EVIDENCE=../cobo_web_design/docs/ai-cache/template-applicable-to-phase-b-activation-readiness-2026-08-29/
```

## Diff

- `applicable_to.go` — activation blockers collector
- `service.go` — readiness + Activate + Upsert PrepareApplicableTo
- `applicable_to_activation_test.go` — NEW

```text
go test ./internal/disclosure/app/ PASS
READY_FOR_PHASE_C=true
DEV_DEPLOY_PERFORMED=false
```
