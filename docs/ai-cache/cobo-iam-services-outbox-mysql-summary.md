# Outbox MySQL wiring — summary

## Package

- `internal/platform/outbox/mysql` — `Repository` implements `outbox.Repository` voi `*sql.DB`.

## Hanh vi

- **Insert**: row `status=pending`, `available_at` tu publisher.
- **LockPendingBatch**: transaction — `SELECT event_id ... WHERE status='pending' AND available_at<=? ORDER BY available_at, event_id LIMIT n FOR UPDATE SKIP LOCKED`; `UPDATE ... SET status='processing'`; `SELECT` full rows; `COMMIT`.
- **MarkProcessed** / **MarkRetry**: nhu in-memory semantics; `last_error` cat 1024 byte.

## Wire

- `cmd/api`: neu mo duoc MySQL pool → `outboxmysql.NewRepository(pool)`; khong thi `outboxinmem`.
- `cmd/worker`: cung logic; **chi** in-memory moi `SeedBootstrapEvents` (tranh trung PK `evt_bootstrap_001` tren DB).

## Yeu cau

- Migration `0001_init_core` bang `outbox_events`.
- DSN can `parseTime=true` (TIMESTAMP).
- MySQL **8.0+** cho `FOR UPDATE SKIP LOCKED`.

## Chua lam

- Ghi outbox trong **cung transaction** voi business (can tx inject vao handler/service).
- Dead-letter `failed_permanent`, jitter backoff (processor van dung backoff so trong code).
