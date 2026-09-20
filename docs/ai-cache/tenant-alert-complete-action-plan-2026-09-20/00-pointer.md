# Tenant alert complete action — pointer (2026-09-20)

Mirror of FE pack `cobo_web_design/docs/ai-cache/tenant-alert-complete-action-plan-2026-09-20/`.

| Field | Value |
|---|---|
| Case | B — SubmitRecord ≠ publish; ConfirmDeadlineAlert = Tenant complete |
| BE code | regression tests only (`submit_record_no_publish_test.go`, `tenant_complete_semantics_test.go`) |
| Migration | false |
| Stage/commit/push/prod | false |

```text
EXTERNAL_PUBLICATION_SIDE_EFFECT=false
LEGACY_ENDPOINT_NAME_MISMATCH=true  # /submit naming
COMPLETE_ENDPOINT=POST /api/v1/company/deadline-alerts/{id}/confirm
```
