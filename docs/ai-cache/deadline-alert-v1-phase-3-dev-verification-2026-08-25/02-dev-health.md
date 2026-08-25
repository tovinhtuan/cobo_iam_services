# 02 — DEV health

```text
HEALTHZ=200
READYZ=200
NGINX_LOGIN_PASSWORD_KEY_VIA_3000=200
```

Containers after deploy: api/worker recreated; mysql/redis/web healthy; no restart loop observed in deploy window.

```text
PANIC_COUNT=0 (deploy+verify window)
FATAL_COUNT=0
```
