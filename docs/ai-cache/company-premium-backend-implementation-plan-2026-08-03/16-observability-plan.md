# Observability plan

Follow existing slog/httpx patterns; avoid new APM dependency.

Suggested counters (if metrics sink exists; else structured logs):
- `company_plan_resolve_total{source,code}`
- `company_plan_resolve_error_total`
- `company_plan_unknown_code_total`

Logs: `company_id`, `plan_code`, `plan_source`, `request_id`; **never** payment artifacts.
