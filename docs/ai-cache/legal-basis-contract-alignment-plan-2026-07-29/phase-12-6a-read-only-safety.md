# Phase 12.6A — Read-only safety

- DB engine / version: 8.0.46 (credentials never logged)
- Database name: cobo_iam
- Host alias: 127.0.0.1
- Port: 3306
- Docker service/network: mysql (cobo-iam-mysql) (compose network cobo-net / published host port)
- Username (masked): c***
- Credential source: docker-compose.dev.yml:mysql published 127.0.0.1:3306 / db=cobo_iam / user=c***
- App credential allowed for 12.6A with guards: true
- SQL allowlist enforced: true
- @@transaction_read_only (inventory tx): 1
- @@session.transaction_read_only: 0
- Write privilege detected on account: true
- Read-only transaction used for all inventory SELECTs: true
- Database mutations: 0
- Tool flags: no --apply; no write repository import

## Grants (privilege names only; passwords stripped)

```
GRANT USAGE ON *.* TO `c***`@`%`
GRANT ALL PRIVILEGES ON `cobo\_iam`.* TO `c***`@`%`
```
