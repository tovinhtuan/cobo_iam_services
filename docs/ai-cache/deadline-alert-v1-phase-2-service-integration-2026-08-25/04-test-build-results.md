# 04 — Test / build results

```bash
go test ./internal/deadlinealerts/... -count=1
go build -o /dev/null ./cmd/api/
```

```text
ok  internal/deadlinealerts/app
ok  internal/deadlinealerts/infra/mysql
ok  internal/deadlinealerts/transport/http
BACKEND_BUILD=PASS
EXIT_CODE=0
DEV_VERIFICATION=NOT_PERFORMED
```
