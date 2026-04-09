# API flow-sequence document summary

- Tai lieu chinh: `docs/api-flow-sequence.md`.
- Pham vi: tat ca endpoint dang register trong `internal/httpserver/server.go`.
- Noi dung:
  - Tong quan middleware/wiring theo `MYSQL_DSN`.
  - Danh sach day du endpoint (health, auth, me, internal authorize, disclosure, workflow, notification, admin).
  - Flow + Mermaid sequence cho tung nhom API, gom:
    - Auth login/refresh/logout/select/switch
    - Me queries
    - Internal authorize/batch
    - Disclosure (co idempotency cho submit/confirm)
    - Workflow
    - Notification (tx enqueue + outbox)
    - Admin APIs (authorize + audit pattern)
  - Mapping nhanh endpoint -> flow pattern.
  - Luu y van hanh (`X-Request-Id`, `Idempotency-Key`, outbox worker).

- Muc dich: dung nhu tai lieu onboarding cho dev/QA va cross-check runtime behavior.
