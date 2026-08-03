# Recommended API contract — RECOMMENDED_PENDING_APPROVAL

## Additive JSON (snake_case)
```json
{
  "plan": {
    "code": "Premium",
    "display_name": "Premium",
    "status": "ACTIVE",
    "source": "member_max_entitlement"
  }
}
```

### Field rules (proposed; Product must lock)
| Field | Rule |
|-------|------|
| `code` | Normalized `Free` \| `Premium` \| `Enterprise` (match entitlement constants) |
| `display_name` | **Server echo of code** initially (FE i18n may override later) — Product lock |
| `status` | Until billing table exists: derive `ACTIVE` if effective window ok; else omit plan. Trial/Expired/Suspended **blocked** without SoT |
| `source` | Required during Case B interim: `member_max_entitlement` vs future `company_subscription` |

## Null / Free semantics (pending Product)
**Interim recommendation for implementation gate:**
- Entitlement empty / unknown → **omit** `plan`
- Entitlement `Free` → omit `plan` **or** include `code=Free` — Product picks; FE badge shows only `Premium` (and optionally `Enterprise` if approved)
- Never invent Premium

## Placement
1. `GET /api/v1/admin/company` top-level `plan`
2. Each item in `GET /api/v1/me/companies` optional `plan`

## Consistency invariant
`GetOwnCompany.plan` ≡ `/me/companies[company_id].plan` for same company at same data version (same reader).

## Backward compatibility
Additive field; old FE ignores. New FE fail-closed if missing.
