---
name: notification-delivery
description: Dùng khi sửa thông báo trong cobo_iam_services - email SMTP (Mailpit khi dev), template notification/templates/*/vi, in-app notification, outbox notification.dispatch, rule notification, chống gửi trùng (idempotency), retry budget, timeout SMTP, không log PII.
---

# Notification Delivery

## Components (verify; as of 2026-10)
- `internal/notification`: embedded templates `templates/*/vi`, SMTP adapter,
  delivery attempts, metric `cobo_email_delivery_total`. Env: `SMTP_*`,
  `EMAIL_DELIVERY_PATH`, `EMAIL_FORMAT`, `EMAIL_SHADOW_MODE`,
  `EMAIL_TEMPLATE_SOURCE`, `EMAIL_NOTIFICATION_ENABLED`,
  `NOTIFICATION_RULES_CONSUMER_ENABLED`.
- Email dedupe: unique `uk_email_notifications_idempotency`; outbox retry
  budget 1m/5m/15m/1h/6h.
- `internal/inappnotification`: `user_in_app_notifications`, plain INSERT,
  **no dedupe**; some creations are fire-and-forget goroutines.
- Dev: Mailpit UI http://localhost:8025 (SMTP 1025) via `docker-compose.dev.yml`.

## Workflow
1. Define trigger, recipients (resolved server-side, same company, active
   membership), channel(s), template, idempotency key.
2. Emit via outbox inside the business transaction; never send email inside
   the request path or inside a DB transaction.
3. Idempotency: email unique key; add a dedupe key for in-app notifications
   when retried paths can create them.
4. SMTP: dial + send deadline; classify retryable (timeouts, 4xx SMTP) vs
   permanent (invalid address).
5. Templates: Vietnamese text UTF-8 (run `vi-text-encoding` checks), escape
   user content in HTML templates, absolute links built from
   `PUBLIC_WEB_BASE_URL`, no tokens in links unless single-use and short-lived.
6. Logging: recipient masked, no full payload, no tokens.
7. Verify in Mailpit locally; check `EMAIL_SHADOW_MODE` behavior on dev.

## Tests
- Duplicate outbox delivery → one email, one in-app item.
- SMTP timeout → retry scheduled, no duplicate after success.
- Recipient from another company excluded.
- Template renders with long Vietnamese names and HTML-special characters.

## Output format
- Trigger → channel → template map
- Idempotency keys
- Retry/timeout policy
- Tests and Mailpit evidence
