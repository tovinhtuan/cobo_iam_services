# Template ApplicableTo Phase D — IAM pointer

```text
PHASE=D
PRIMARY_EVIDENCE=../cobo_web_design/docs/ai-cache/template-applicable-to-phase-d-version-clone-api-wiring-2026-08-29/
```

## Diff

- `applicable_to.go` — UnmarshalJSON / preserve helpers / clone CLEAR
- `clone_template.go` — ApplyCloneApplicableToDefaults
- `service.go` — preserve on Upsert + UpdateTemplateDeadlineConfig
- `applicable_to_phase_d_test.go` — NEW

```text
go test ./internal/disclosure/app/ PASS
READY_FOR_PHASE_E=true
DEV_DEPLOY_PERFORMED=false
```
