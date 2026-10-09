---
name: backend-security-audit
description: Dùng khi audit/harden bảo mật cobo_iam_services (Go) - authentication/authorization bypass, IDOR, tenant isolation, SQL injection, SSRF, input validation, body size, JWT validation, rate limiting, sensitive logging, credential exposure, govulncheck, Redis/worker/outbox, /metrics và internal endpoint.
---

# Backend Security Audit

## When to use

- Changes to login, refresh, logout, password, token, session, company, role
  or permission code.
- New/changed public or internal endpoints, migrations, queries, cache,
  workers, integrations.
- Files, URLs, templates, email, exports, webhooks, OAuth (template builder).
- Config, secrets, Docker, deployment, logging or audit changes.
- Security review, threat modeling, hardening, vulnerability triage.

Defensive review only. No production exploitation, credential collection,
load testing against shared environments, or probing unrelated systems.

## Known hotspots (verify; as of 2026-10)

- Auth is per-handler; no auth middleware. Grep new handlers for the token +
  permission check.
- Opaque access tokens (deployed mode) have no enforced expiry and live in an
  unbounded in-process map.
- No app-level rate limiting; `login_attempts` not enforced; only nginx limits
  `listed-lookup`.
- Most JSON bodies and multipart uploads lack `http.MaxBytesReader`.
- `/metrics` trusts loopback/private IPs (nginx forwards from a private IP);
  `X-Internal-Token` compared non-constant-time there.
- `X-Request-Id` accepted from client unvalidated (log injection).
- SMTP sends without deadline.
- Repo-tracked dev secrets: `configs/login_password_rsa_dev.pem`, inline DB
  credentials in `docker-compose.artifacts.yml`, seed passwords in comments.
- Outbox `notification.dispatch` handler logs the full payload.

## Workflow

1. Read repo rules, relevant `docs/ai-cache/`, README, affected packages,
   migrations and tests.
2. Assets, actors, trust boundaries, privileged operations, sensitive data,
   abuse cases.
3. Trace request/event paths: handler → authn/authz → service → repository →
   DB/cache/external → outbox → worker.
4. Review:
   - **Authn/authz:** token inspection present on every non-public route;
     permission + company scope from the token; platform vs tenant admin
     (`cobo-admin-role-guard`); IDOR on every ID in path/query/body;
     privilege escalation via role/permission assignment.
   - **JWT checklist:** allowed-alg allowlist (reject `none` and alg
     confusion), `iss`/`aud`/`typ` enforced, `exp` **required** (not only
     checked when present), `nbf`/`iat` with bounded clock skew, key source
     and rotation (`JWT_VERIFY_PUBLIC_KEYS_JSON` currently unused), refresh
     token rotation and reuse detection, revoke on logout/password change.
   - **Input:** SQL parameterization, dynamic identifiers via allowlist,
     pagination caps, body/upload size, file type sniffing, URL fetch (SSRF:
     scheme/host allowlist, block private ranges, no redirects to internal),
     template rendering.
   - **Transactions/races:** status guards, idempotency, duplicate worker
     side effects.
   - **Redis:** key isolation per tenant, staleness after permission change,
     fail-open behavior on privileged paths.
   - **Migrations/seeds:** default credentials, seed data on shared envs.
   - **Secrets:** config, env, logs, errors, metrics, Docker, repo files
     (see `secrets-cors-cookie-review`).
   - **Abuse controls:** rate limits, timeouts, resource exhaustion.
   - **CORS, internal endpoint auth, TLS assumptions, error detail.**
5. Automated checks:
   ```bash
   go vet ./...
   go run golang.org/x/vuln/cmd/govulncheck@latest ./...
   python3 .claude/skills/secrets-cors-cookie-review/scripts/scan_secrets.py .
   ```
   (`govulncheck` needs network; otherwise report `BLOCKED:`.)
6. Classify Critical/High/Medium/Low/Info with file:line, preconditions,
   impact, exploitability, focused fix.
7. Regression tests for the vulnerable boundary without real secrets;
   re-check in-memory and MySQL paths.

## Guardrails

- Never print passwords, JWTs, private keys, DSNs, cookies or bearer headers.
- Backend authorization is the source of truth.
- Do not weaken authn/authz for local testing.
- No insecure fallback credentials; no global TLS/CORS disabling.
- Do not silently change migration semantics, token lifetime or permission
  defaults.
- Prefer parameterized queries, allowlists, bounded inputs, explicit errors,
  fail-closed for privileged operations.

## Required report

```text
Assets and trust boundaries:
Threats reviewed:
Automated check results (vet, govulncheck, secret scan):
Critical/High findings:
Medium/Low findings:
Authorization and tenant-isolation result:
JWT/session result:
Tests or static checks:
Residual risks and follow-up:
```

## Verification

```text
go test ./...
go vet ./...
docker compose -f docker-compose.dev.yml build api
```
Add worker/full-stack checks when affected. Report `BLOCKED:` with reason.
