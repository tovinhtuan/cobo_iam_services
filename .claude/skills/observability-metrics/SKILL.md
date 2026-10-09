---
name: observability-metrics
description: Dùng khi thêm hoặc review log, metric, audit trong cobo_iam_services - slog JSON, request id, Prometheus metric prefix cobo_, label cardinality, DB backlog gauge, alert rules deploy/monitoring, /metrics guard, audit_logs cho thao tác đặc quyền; không lộ PII/secret.
---

# Observability & Metrics

## Current state (verify; as of 2026-10)
- Logging: `log/slog` JSON to stdout (`internal/platform/logger`); request id
  via `X-Request-Id` (client-supplied value accepted unvalidated).
- Metrics: Prometheus `client_golang`, prefix `cobo_`: `cobo_reminder_*`,
  `cobo_email_delivery_total`, `cobo_adhoc_*`, `cobo_dashboard_overview_*`,
  DB gauges `cobo_email_backlog`, `cobo_reminder_backlog`,
  `cobo_outbox_stale_processing`, `cobo_reminder_stuck_dispatching`.
- `/metrics` on the API only (loopback/private IP or `X-Internal-Token`);
  worker exposes no metrics endpoint.
- No per-request HTTP metrics middleware.
- Alerts: `deploy/monitoring/reminder_alert_rules.yml`.
- Audit: append-only `audit_logs` (`internal/audit`), CMS audit actions.

## Rules
Logs
- Structured fields: `request_id`, `company_id`, `membership_id` (IDs only),
  `module`, `op`, `err`. No emails/phones/names unless masked; never tokens,
  passwords, DSNs, full payloads.
- Validate/limit incoming `X-Request-Id` (length, charset) before logging.
- One error log per failure at the boundary that handles it.

Metrics
- Name `cobo_<module>_<thing>_<unit>`; counters end in `_total`,
  durations `_seconds` histograms.
- Labels: low cardinality only (status, outcome, channel, route template).
  Never user/company IDs, emails, raw paths.
- For async flows add backlog gauge + failure counter + age of oldest item.
- Register once (package init or constructor guarded), test with
  `prometheus/testutil`.

Audit
- Privileged mutations (role/permission, membership, company status,
  break-glass, delegation, config apply/rollback, CMS publish) write
  `audit_logs` with actor, company, target, action, before/after summary.

Alerts
- Each new backlog/failure metric gets an alert rule or an explicit note why
  not; keep rules in `deploy/monitoring/`.

## Output format
- Signals added (logs/metrics/audit) and why
- Cardinality check
- Alert rules
- Tests
