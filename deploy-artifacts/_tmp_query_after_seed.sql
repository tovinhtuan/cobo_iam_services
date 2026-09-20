SELECT occurrence_id, status, last_error_code, attempt_count, scheduled_at, updated_at
FROM reminder_occurrences
WHERE occurrence_id LIKE 'idem-smoke-dash-act%' OR idempotency_key LIKE 'idem-smoke-dash-act%'
ORDER BY scheduled_at DESC LIMIT 5;

SELECT id, user_id, company_id, kind, title, body, resource_type, resource_id, created_at
FROM user_in_app_notifications
WHERE company_id='c_001' AND created_at > '2026-09-20 06:40:00'
ORDER BY created_at DESC LIMIT 10;
