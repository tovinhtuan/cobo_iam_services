# API contract — actual (Phase 2)

## Endpoint
`GET /api/v1/disclosure-types/{type_id}` — auth/RBAC unchanged.

## Additive field
```json
{
  "resolved_deadline_rule": {
    "rule_code": "DEFAULT",
    "rule_label_key": "deadline.rule.default",
    "resolution_source": "DEFAULT_TEMPLATE_RULE",
    "resolved_days": 23,
    "day_type": "WORKING_DAYS",
    "base_date_source": "CYCLE_START",
    "periodicity": "monthly",
    "due_date": "2026-07-31"
  }
}
```

## NO_RULE shape
```json
{
  "resolved_deadline_rule": {
    "resolution_source": "NO_RULE",
    "day_type": "WORKING_DAYS",
    "base_date_source": "CYCLE_START",
    "periodicity": "monthly"
  }
}
```
(`resolved_days` / `due_date` omitted; `rule_code` omitted when empty.)

## Presence
Object set when `applicability_rules` present **and** company profile load succeeds. Omit if profile lookup fails (fail-safe, no 500).

## Existing fields
`deadline_rule`, `deadline_summary`, `applicability_rules`, HTTP status — unchanged.
