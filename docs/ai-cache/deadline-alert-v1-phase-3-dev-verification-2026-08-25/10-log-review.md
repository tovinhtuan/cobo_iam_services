# 10 — Log review

Window: ~15m after BE deploy + API/browser verification.

```text
PANIC_COUNT=0
FATAL_COUNT=0
DEADLINE_ALERT_QUERY_ERRORS=0
UNEXPECTED_5XX=0
```

`docker logs cobo-iam-api --since 15m`: startup INFO only (modules enabled, api listening). No SQL/scan errors matched.
