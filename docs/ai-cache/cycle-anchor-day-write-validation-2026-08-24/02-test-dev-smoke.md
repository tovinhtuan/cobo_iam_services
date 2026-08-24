# 02 — Tests + DEV smoke

## Unit / service

```text
go test ./internal/disclosure/app/ -run 'ValidateCycleAnchorDay|CycleAnchorDay|ClearCycleAnchor|ClampDayOfMonth'
→ PASS

go build ./cmd/api → PASS
docker compose -f docker-compose.dev.yml build api → PASS (exit 0)
```

## DEV deploy

```text
bash deploy-dev.sh be --skip-tests → PASS
healthz=200 readyz=200
```

## API smoke (DEV)

| Call | Result |
|------|--------|
| CMS PUT config day=29/30/31 | 200 |
| CMS PUT config day=0 | 200 (unset) |
| CMS PUT config day=32/-1/100 | 400 INVALID_REQUEST |
| Company PATCH day=31 | 200 |
| Company PATCH day=32 | 400 |
| Company clear (+ day=32 payload) | 200 |

Note: MySQL `RowsAffected=0` when JSON unchanged can surface as 404 on identical PUT — preexisting repo quirk; unrelated to this delta.
