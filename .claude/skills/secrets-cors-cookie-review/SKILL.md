---
name: secrets-cors-cookie-review
description: Dùng khi review CORS & credentials cho Cobo - CORS allowlist, preflight, Allow-Credentials, cookie HttpOnly/Secure/SameSite, hardcoded secrets/API key/DB credential trong repo, git history, bundle dist, log, Docker compose; secret management, rotation, VITE_* và vite define nhúng lúc build. Có script quét secret.
---

# Secrets, CORS & Cookie Review

## When to use
- Changes to CORS, auth transport (header vs cookie), nginx, compose, env,
  deploy scripts, build config (`vite.config.ts`, `VITE_*`).
- Adding any credential, key, token, DSN or third-party API.
- Pre-release security review; suspected leak.

Defensive only. Never print secret values; the script redacts them.

## Current state (verify; as of 2026-10)

**CORS** - `cobo_iam_services/internal/httpserver/cors.go`
- `CORS_ALLOWED_ORIGINS` set → exact-match allowlist, reflects origin,
  `Vary: Origin`. Unset + `ENV=development` → any loopback origin +
  `PUBLIC_WEB_BASE_URL`. Otherwise deny.
- No `Access-Control-Allow-Credentials` (tokens travel in `Authorization`).
- The dev server stack (`docker-compose.artifacts.yml`) runs `ENV=development`.
- In production the SPA and API are same-origin behind nginx, so CORS should
  be deny-by-default there.

**Cookies** - none. Access/refresh tokens are returned in JSON and stored in
`localStorage` by the web app.

**Known secret hotspots (do not copy values into reports)**
- `cobo_iam_services/configs/login_password_rsa_dev.pem` (private key, likely tracked).
- `cobo_iam_services/docker-compose.artifacts.yml` inline DB credentials and DSNs.
- `cobo_iam_services/migrations/seed_*.sql` comments with shared dev passwords.
- `cobo_web_design/smoke-qa-create-type.mjs` hardcoded login; other E2E scripts
  use seed credentials and a hardcoded dev server IP.
- `vnstock/pipeline/README.md` and `vnstock/deploy.sh` server/DB details.
- `cobo_web_design/vite.config.ts` `define: process.env.GEMINI_API_KEY`.
- Notification outbox handler logs full payload.

## Workflow

1. **Scan working tree and build output**
   ```bash
   python3 .claude/skills/secrets-cors-cookie-review/scripts/scan_secrets.py .
   npm run build && python3 .claude/skills/secrets-cors-cookie-review/scripts/scan_secrets.py dist   # web
   git ls-files | grep -Ei '\.(pem|key|p12|env)$|local\.env'                                          # tracked secret files
   ```
2. **Scan history** (read-only):
   ```bash
   git log -p --all | python3 .claude/skills/secrets-cors-cookie-review/scripts/scan_secrets.py --stdin
   ```
   A secret in history is leaked even if deleted now → rotate. History
   rewrite (filter-repo) needs explicit user approval; never do it yourself.
3. **Build-time exposure (web)**: every `VITE_*` and every `define` value is
   public. Confirm none is a secret; remove dead `define` entries.
4. **CORS review**
   - Exact allowlist per environment; no `*`, no regex/suffix match, no
     reflecting arbitrary `Origin`, `null` origin rejected.
   - `Access-Control-Allow-Credentials: true` only with an exact origin, never
     with wildcard, and only if cookies are used.
   - Preflight: allowed methods/headers minimal; `Access-Control-Max-Age` set
     deliberately; OPTIONS must not reach handlers with side effects.
   - `Vary: Origin` present when reflecting.
   - Non-dev environments must not run with `ENV=development`.
5. **Cookie review** (if introducing cookies, e.g. refresh token):
   - `HttpOnly; Secure; SameSite=Strict` (or `Lax` with justification);
     narrow `Path` (e.g. `/api/v1/auth`); `__Host-` prefix when possible.
   - CSRF defense for cookie-authenticated mutations (SameSite + custom
     header or token); CORS credentials only for the exact web origin.
   - Logout/rotation clears the cookie; no token in both cookie and body.
6. **Secret management**
   - Source: env vars only (`internal/platform/config`), `.env.example` with
     placeholders, real values in gitignored files or the server.
   - Startup guard `validateSecurityCriticalConfig` should reject defaults for
     every signing secret, not only some.
   - No secrets in logs, errors, metrics labels, Docker image layers,
     `docker inspect`-visible compose files committed to git.
7. **Rotation playbook** (per leaked item): generate new → deploy to server
   env → restart API/worker → verify → revoke old (sessions/keys/DB user) →
   document in ai-cache without the value.

## Header checks for deployed web
```bash
bash .claude/skills/cache-versioning-review/scripts/check_http_headers.sh http://<host>:3000
```
Expect (target state): `Content-Security-Policy`, `X-Content-Type-Options:
nosniff`, `X-Frame-Options: DENY` or CSP `frame-ancestors`, `Referrer-Policy`,
HSTS when served over HTTPS.

## Guardrails
- Never echo, commit, screenshot or paste secret values; refer by file:line
  and variable name.
- Do not rotate, revoke or rewrite history without explicit approval.
- Do not "fix" CORS by widening it.

## Required report
```text
Secret scan (tree / dist / history):
Tracked secret files:
Build-time exposure:
CORS result per environment:
Cookie/CSRF result:
Security headers:
Findings by severity (file:line, no values):
Rotation needed:
```
