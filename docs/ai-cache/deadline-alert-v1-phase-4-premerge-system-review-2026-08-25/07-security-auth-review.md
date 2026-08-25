# 07 — Security / auth review

```text
authorizeView → Action deadline.view
ListRows: dr.company_id = ? + BuildListRowsScopeSQL(accessScope)
Service: AllowsRow re-check after ListRows
```

EXISTS/NOT EXISTS on `periodic_cycles` cannot expand company scope (filtered via `dr` already company-bound).

```text
AUTHORIZATION_REVIEW=PASS
AUTHORIZATION_MODEL_CHANGED=false
CROSS_COMPANY_RISK=PASS (Phase 3: c_002 403 / no QA leak)
```
