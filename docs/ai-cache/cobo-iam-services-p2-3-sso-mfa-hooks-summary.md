# P2.3 SSO/MFA extension hooks — summary

## Interfaces (`internal/iam/app/hooks.go`)

- **SSOLoginBridge** — `TryExternalPrimaryAuth(ctx, req LoginRequest) (user *AuthenticatedUser, handled bool, err error)`. `handled=true` bo qua `CredentialVerifier`.
- **MFACheck** — `VerifyAfterPrimaryAuth(ctx, user, req LoginRequest) error`. Chay sau primary auth (SSO hoac password) va sau khi user `active`, **truoc** `GetMembershipsByUser` va issue token/session.

## Wiring

- `NewService(cred, sessions, tokens, memberships, idgen, opts ...ServiceOption)`
- `WithSSOLoginBridge(b)`, `WithMFACheck(m)` — hook nil-safe.

## Request shape

- `LoginRequest`: them JSON `mfa_otp`, `extensions`; `login_id`/`password` co tag JSON chuan; IP/UserAgent `json:"-"` (server-set).

## Errors

- `perr.CodeMFARequired` cho implementer MFA tra ve `NewHTTPError` (goi y HTTP 403).

## Contract

- `iamapp.Service` interface khong doi; chi them optional constructor options.
