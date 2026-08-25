# 08 — API / FE compatibility

```text
ENDPOINT=GET /api/v1/company/deadline-alerts
API_CONTRACT_CHANGED=false
API_SCHEMA_CHANGED=false
FE_SOURCE_CHANGED=false
FE_SOURCE_FILES_IN_COMMIT=0 (recommended candidate)
```

Behavior change: list membership (Draft obligations appear; post-submit disappear).

Phase 3: `BROWSER_RENDERS_ACTIONABLE_DRAFT=PASS`.

```text
EXPECTED_ALERT_COUNT_DIRECTION=INCREASE (intended)
BACKWARD_COMPATIBILITY_RISK=LOW
```
