# Create your account — public register flow (FE + IAM)

**Updated:** 2026-05-25  
**Scope:** `cobo_web_design` (`/register`, "Create your account") ↔ `cobo_iam_services` (`POST /api/v1/auth/register`)

## Entry

| Layer | Location |
|-------|----------|
| FE route | `/register` → `RegisterPage.tsx` |
| FE API | `authApi.registerPublic` → `POST /api/v1/auth/register` |
| BE handler | `internal/iam/transport/http/handler.go` → `RegisterPublic` |
| BE service | `internal/iam/app/service.go` → `RegisterPublic` |
| DB | `internal/iam/registrationmysql/register_public.go` |

Not the same as CMS admin **Create account** (`cms-core/pages.tsx` → platform admin user provisioning).

## FE steps

1. Form: email, full_name, optional company_name, password (≥12), confirm_password.
2. Client validation: min password, match confirm.
3. `useAuth().register` → `App.handleRegister` → `registerPublic`.
4. Password: optional RSA-OAEP-256 via `GET /api/v1/auth/login-password-key` (same as login).
5. On success: `applyNormalizedLoginResponse` (tokens, bootstrap `/me`, redirect via route guard).

## BE steps

1. Guard: `REGISTRATION_DISABLED=true` → 403; no `WithPublicRegistration(db)` → 503.
2. Validate email, full_name, password ≥12, password == confirm_password.
3. bcrypt hash password.
4. **With company_name:** `RegisterPublicAccount` — user + company + membership + roles **`self_reg_company_owner`** (global) **and** tenant **`admin_doanh_nghiep`**; `grantRoleCompanyProfilePermissionsTx` ensures `company.view`/`company.edit` on both role surfaces at register time; company `verification_status=unverified`; tenant roles `admin_doanh_nghiep`, `user_thuong` seeded on company row.
5. **Without company_name:** `RegisterPublicUserOnly` — user + credential + Free tier only.
6. Auto `Login` with same credentials → `LoginResponse` (201).
7. `issueEmailVerificationOTP` → outbox `auth.email_verification_requested` (worker sends email).

## Post-register navigation (FE)

| IAM `next_action` | Typical case | FE destination |
|-------------------|--------------|----------------|
| `load_effective_access` | 1 company membership (self-reg with company) | `/app/dashboard` (hint `enterprise`) |
| `no_company_onboarding` | 0 memberships (user-only signup) | `/app/no-company` |
| `select_company` | Multiple memberships + `pre_company_token` | `CompanySelectionPage` |

CMS redirect if user has `platform.cms.view` (strict CMS permissions).

## Side effects

- Audit: `register_public_success` / `register_public_failure`
- Event: `iam.user.registered`
- Portal banner when `email_verified=false` (`PortalLayout`)

## Registration OTP email template (2026-05-25)

- Template key: `auth.email_verification` (`internal/notification/templates/auth.email_verification/`)
- Vars: `otp_code`, `expiry_minutes`, `support_email` (`SUPPORT_EMAIL`), `website_url` (`PUBLIC_WEB_BASE_URL`)
- Legacy path (`EMAIL_TEMPLATE_SOURCE=legacy`): same copy via `registrationOTPEmailContent` in `service.go`

## Email verify UX (2026-05-25)

- BE: `email_verified` always serialized (no `omitempty`); `enrichLoginVerification` sets flag for no-company and multi-company logins.
- FE: `resolvePostLoginPath` → `/app/verify-email` when `user.emailVerified === false` (register + login).
- Screen: `/app/verify-email` (`VerifyEmailPage`) — OTP + resend; after success → dashboard or `/app/no-company`.

## Related docs

- `cobo_iam_services/docs/lifecycle-flow-inventory.md` (Auth lifecycle)
- `cobo_iam_services/docs/ai-cache/login-password-encryption-setup-summary.md`
