# 04 — Test / build regression

```bash
go test ./internal/deadlinealerts/... -count=1
go build -o /dev/null ./cmd/api/
```

```text
LOCAL_REPOSITORY_TESTS=PASS
LOCAL_SERVICE_TESTS=PASS
BACKEND_BUILD=PASS
EXIT_CODE=0
```

Phase 4 did not re-run DEV E2E (Phase 3 evidence accepted; source unchanged since).
