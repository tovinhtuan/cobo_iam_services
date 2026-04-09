# P1 MySQL + httpserver + transactional notification — summary

## Mục 1 — Integration HTTP

- `internal/httpserver` — `New(ctx, Deps{Log, Config, DB})` tra ve `http.Handler` + `cleanup()` (Redis).
- `cmd/api/main.go` chi con config, mo MySQL tuy chon, `defer pool.Close()`, goi `httpserver.New`.
- Test: `internal/httpserver/server_test.go` — `healthz`, `readyz` khi DB nil, login `single@example.com`.

## Mục 2 — Transactional outbox (notification)

- `internal/platform/outbox/mysql/tx.go`: `InsertTx`, `PublishEventTx`.
- `notificationapp.TxJobRepository` + `CreateJobTx` tren `notification/infra/mysql`.
- `notificationapp.WithTransactionalEnqueue(db, outboxMysqlRepo)` — `EnqueueNotification` dung `BeginTx` + `CreateJobTx` + `PublishEventTx` + `Commit` khi du dieu kien.

## Mục 3 — P1 persistence MySQL

- Migration `0004_p1_business_tables.{up,down}.sql`: `disclosure_records`, `workflow_instances`, `workflow_tasks`, `notification_jobs`, `notification_deliveries`.
- Repo: `disclosure/infra/mysql`, `workflow/infra/mysql`, `notification/infra/mysql`.
- `httpserver.register`: neu `DB != nil` dung MySQL repos; notification nhan `WithTransactionalEnqueue` khi outbox MySQL.

## Luu y van hanh

- Bang P1 FK toi `companies` — can co company hop le trong DB (fixture / seed) truoc khi insert disclosure/workflow/notification.
- Authz van in-memory; membership query van in-memory — user `u_123`/`m_001`/`c_001` khop fixture.
