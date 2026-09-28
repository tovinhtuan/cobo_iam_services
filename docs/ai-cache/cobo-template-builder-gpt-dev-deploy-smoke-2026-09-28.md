# CoBo Template Builder GPT — DEV deploy and smoke

Created: 2026-09-28  
Scope: DEV only (`88.216.208.0`); production was not accessed.

## Outcome

The dormant Template Builder OAuth implementation is deployed to DEV. The
database expansion migration is present and the web authorization page is
available. OAuth remains deliberately disabled because the DEV API does not
yet have a configured public HTTPS origin suitable for a ChatGPT Action.

## Migration validation

- MySQL: DEV 8.0.46.
- Isolated schema `cobo_template_builder_oauth_test_20260928` was confirmed
  absent, created, and used only for this verification.
- `0150_template_builder_oauth_authorization_codes.up.sql` succeeded twice.
- The expected table existed exactly once after rerun.
- The down migration removed that table; the isolated schema was then dropped.
- `0150_template_builder_oauth_authorization_codes.up.sql` was applied to
  `cobo_iam` DEV and recorded in `schema_migrations`.

## Deployment and smoke

- Backend API/worker rebuilt and restarted from the current `fixbug/test-9`
  working tree.
- Frontend rebuilt and the DEV web container restarted.
- `GET /healthz`: 200.
- `GET /readyz`: 200.
- `GET /oauth/template-builder/authorize`: 200 (SPA authorization screen).
- `GET /api/v1/oauth/template-builder/authorize`: 404 while the feature flag
  is off, which is the expected fail-closed state.
- OpenAPI was corrected to use the real `/api/v1/...` routes and no longer
  advertises an unimplemented rate-limit response.

## Safety state

- `TEMPLATE_BUILDER_OAUTH_ENABLED`: not enabled or changed during this run.
- No OAuth signing secret, OAuth client id, redirect URI, token, or password
  was read into this report.
- No CMS import validation, confirmation, draft creation, publish, activate,
  email, or production operation was performed.

## Remaining release gate

ChatGPT Actions require a public HTTPS origin for the authorization, token,
and validation endpoints. The current DEV address is HTTP-only
`http://88.216.208.0:8080`; it must not be placed into the GPT Action or OAuth
configuration. Before enabling the flag, provide or configure a DEV TLS
reverse-proxy/domain, then set the client id, exact ChatGPT redirect URI and a
new 32-byte-or-longer signing secret outside the repository. Re-run OAuth
authorization-code/PKCE smoke after those values are configured.

