---
name: backend-security-audit
description: Dùng khi audit hoặc harden bảo mật cobo_iam_services, gồm IAM, JWT/session, authorization, SQL/migration, secrets, Redis, worker/outbox, audit log, SSRF và API abuse controls.
---

# Backend Security Audit

## When to use

Use this skill when a task:

- changes login, refresh, logout, password, token, session, company, role, or permission code;
- adds or changes public/internal API endpoints, migrations, queries, cache, workers, or integrations;
- handles files, URLs, templates, email, exports, webhooks, or external services;
- changes configuration, secrets, Docker, deployment, logging, or audit behavior;
- requests security review, threat modeling, hardening, or vulnerability triage.

This is a defensive review. Do not exploit production, bypass controls, collect
credentials, perform load testing, or probe unrelated systems.

## Workflow

1. Read repository rules, relevant `docs/ai-cache/` context, README, affected
   packages, migrations, and tests.
2. Identify assets, actors, trust boundaries, privileged operations, sensitive
   data, and abuse cases.
3. Trace request and event paths through handler, authn/authz, service,
   repository, database, cache, external system, outbox, and worker.
4. Review:
   - password hashing, token issuance/refresh/revocation, expiry, rotation, and
     replay resistance;
   - tenant/company isolation, role/permission checks, IDOR and privilege
     escalation paths;
   - input validation, SQL parameterization, dynamic identifiers, pagination,
     limits, file/URL handling, SSRF, and deserialization;
   - transaction boundaries, race conditions, idempotency, retries, and
     duplicate worker side effects;
   - Redis key isolation, cache poisoning/staleness, and fail-open behavior;
   - migration safety, seed data, default credentials, and rollback exposure;
   - secrets in config, environment, logs, errors, metrics, traces, and Docker;
   - audit completeness, alertability, rate limits, timeouts, and resource
     exhaustion controls;
   - CORS, internal endpoint authentication, TLS assumptions, and error detail.
5. Classify findings as Critical, High, Medium, Low, or Informational. Include
   file/line evidence, preconditions, impact, exploitability, and a focused fix.
6. Add regression tests for the vulnerable boundary without using real secrets
   or destructive data. Re-check both in-memory and MySQL-backed paths when
   relevant.

## Guardrails

- Never print or copy passwords, JWTs, private keys, DSNs, cookies, or bearer
  headers into reports or test output.
- Backend authorization is the source of truth; frontend visibility is not a
  security control.
- Do not weaken authentication or authorization to simplify local testing.
- Do not add insecure fallback credentials or disable TLS/CORS checks globally.
- Do not silently change migration semantics, token lifetime, or permission
  defaults; document compatibility impact.
- Prefer parameterized queries, allowlists, bounded inputs, explicit error
  classes, and fail-closed behavior for privileged operations.

## Required report

```text
Assets and trust boundaries:
Threats reviewed:
Critical/High findings:
Medium/Low findings:
Authorization and tenant-isolation result:
Tests or static checks:
Residual risks and follow-up:
```

## Verification

Run relevant checks from `cobo_iam_services`:

```text
go test ./...
go vet ./...
docker compose -f docker-compose.dev.yml build api
```

Add worker/full-stack checks when affected. Report `BLOCKED:` with the concrete
reason if a required check cannot run.
