# Login password RSA encryption — setup & troubleshooting

**Updated:** 2026-05-24

## Symptom

```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "login password encryption is not configured"
  }
}
```

From `GET /api/v1/auth/login-password-key` when API has no `LOGIN_PASSWORD_RSA_PRIVATE_KEY_PEM`.

## Fix (dev / artifacts server)

1. Ensure `configs/login_password_rsa_dev.pem` exists on host at `/root/cobo_project/configs/` (`make deploy-be` SCPs it).
2. Set on API container (docker-compose.artifacts.yml):
   - `LOGIN_PASSWORD_RSA_PRIVATE_KEY_PEM_FILE=/app/configs/login_password_rsa_dev.pem`
   - `LOGIN_PASSWORD_RSA_KEY_ID=dev`
3. Restart API: `docker compose -f docker-compose.artifacts.yml up -d --force-recreate api`

## 502 Bad Gateway on `/api/*` via port 3000

Nginx (`cobo-web-design`) proxies to `http://api:8080`. **502 = API container not listening** (crash loop or stopped).

Common cause after enabling RSA env without the PEM file on disk (old API binary exits on `config.Load`).

**Quick recovery on server:**

```bash
cd /root/cobo_project
# Option A: add missing key
ls -la configs/login_password_rsa_dev.pem   # must exist
# Option B: temporarily disable RSA in compose/.env, then:
docker compose -f docker-compose.artifacts.yml up -d --force-recreate api
docker compose -f docker-compose.artifacts.yml logs api --tail 50
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8080/healthz   # expect 200
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8080/api/v1/auth/login-password-key  # 200 or 204
```

## Behaviour after fix

- `GET /api/v1/auth/login-password-key` → `200` + `public_key_spki_b64`
- FE encrypts password; `POST /api/v1/auth/login` sends `password_cipher`

## Fallback (no RSA)

- API returns `204 No Content` on login-password-key when RSA not configured
- FE sends plaintext `password` (current `authApi.login`)
