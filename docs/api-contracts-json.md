# API Contracts — JSON Reference

Tai lieu mo ta body JSON (mau request/response) cho REST API cua `cobo_iam_services`. Chi tiet giai doan trien khai xem `implementation-step-by-step.md`.

**Kem theo (machine-readable):** `api-v1-implemented-contracts.json` — danh sach path/method va schema mau cac endpoint da wiring trong `httpserver` (cong voi `/internal/v1/authorize*`, health).

**OpenAPI / Postman (B1):** `openapi/v1-iam-snapshot.yaml` (build tu JSON bang `docs/scripts/build-openapi-snapshot.mjs`) + `openapi/README.md`.

## Quy uoc

- Base path: `/api/v1` (client), `/internal/v1` (noi bo).
- `Content-Type: application/json`.
- Ung dung **khong** lay token claim roles/permissions lam nguon bao mat duy nhat; backend van authorize lai khi thuc hien action.
- Access token chi nen chua toi thieu: `sub` (user_id), `session_id`, `membership_id`, `company_id`, `iat`, `exp` — khong nhung JSON permission day du.

### Header Authorization (client API)

- `Authorization: Bearer <access_token>`
- Trace: `X-Request-Id` (optional client gui; server generate neu thieu)

### Boc loi thong nhat

Mau loi (HTTP 4xx/5xx):

```json
{
  "error": {
    "code": "PERMISSION_DENIED",
    "message": "Human readable message",
    "details": {}
  }
}
```

Ma loi goi y: `INVALID_CREDENTIALS`, `ACCOUNT_LOCKED`, `SESSION_EXPIRED`, `NO_ACTIVE_COMPANY_ACCESS`, `MEMBERSHIP_NOT_FOUND`, `COMPANY_CONTEXT_REQUIRED`, `COMPANY_SCOPE_MISMATCH`, `PERMISSION_DENIED`, `DATA_SCOPE_DENIED`, `RESPONSIBILITY_REQUIRED`, `STATE_CONFLICT`, `MFA_REQUIRED`.

---

## A. Authentication APIs

### GET /api/v1/auth/login-password-key

Khi cau hinh `LOGIN_PASSWORD_RSA_PRIVATE_KEY_PEM` (PEM khoa RSA 2048+; env `LOGIN_PASSWORD_RSA_KEY_ID` tuy chon, mac dinh `default`), public key duoc dung voi thuat toan `RSA-OAEP-256` (SHA-256) de ma hoa truoc khi login. Goi endpoint nay **truoc** `POST /api/v1/auth/login` de lay khoa public (Web Crypto `spki`).

- **Khoa chua cau hinh:** HTTP `404` + `error` JSON, code `INVALID_REQUEST` (tinh nang ma hoa tren duong thong bao chua bat).

**Response 200**

```json
{
  "kid": "dev-local",
  "alg": "RSA-OAEP-256",
  "public_key_spki_b64": "MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA…"
}
```

`public_key_spki_b64` la PKIX subjectPublicKeyInfo (DER) encode base64 — import Web Crypto: `importKey("spki", ...)` voi `RSA-OAEP` + `SHA-256`. Be mat khau UTF-8 toi da ~**190 byte** (RSA-2048 OAEP-SHA256).

---

### POST /api/v1/auth/login

Dang nhap co the gui **mật khẩu dạng plaintext** hoặc (khi may chu cau hinh khoa RSA) **`password_cipher`**; khong gui dong thoi `password` va `password_cipher` tren cung mot body voi cung nghia uu tien — client front-end chi nen gửi `password_cipher` khi da nhan public key 200.

**Request (plaintext — tuong thich nguoc va script)**

```json
{
  "login_id": "user@example.com",
  "password": "secret",
  "remember_me": true
}
```

Gia tri tuy chon: `email` cung nghia voi `login_id` (FE `cobo_web_design` dung `email`).

Tuy chon (P2.3 hooks): `mfa_otp`, `extensions` (map tuy y cho OIDC/assertion phu).

**Request (mật khẩu mã hóa RSA-OAEP-256, kid khớp may chủ)**

```json
{
  "email": "user@example.com",
  "remember_me": true,
  "password_cipher": {
    "alg": "RSA-OAEP-256",
    "kid": "dev-local",
    "ciphertext_b64": "K7gNU3sdo+OL0wNhqoVWhr3g6QUMw2gWv3s+…"
  }
}
```

- Khi dung `password_cipher`: truong `password` (plaintext) **khong** can gui.
- Neu client gui `password_cipher` nhung may chu chua cau hinh khoa: `400` + `password_cipher is not supported`.
- Sai mã / sai kid (khac `LOGIN_PASSWORD_RSA_KEY_ID`): `400` + `invalid password_cipher` hoac `unknown password_cipher.kid`.

**Response** — cau truc nhu cac mau phia duoi (khong doi khi dung `password_cipher`).

**Response — 1 company active (auto select)**

```json
{
  "user": {
    "user_id": "u_123",
    "full_name": "Nguyen Van A"
  },
  "session": {
    "access_token": "jwt-access",
    "refresh_token": "jwt-refresh",
    "expires_in": 900
  },
  "current_context": {
    "company_id": "c_001",
    "membership_id": "m_001",
    "auto_selected": true
  },
  "next_action": "load_effective_access"
}
```

**Response — nhieu company active (can chon)**

```json
{
  "user": {
    "user_id": "u_123",
    "full_name": "Nguyen Van A"
  },
  "session": {
    "pre_company_token": "jwt-pre-company",
    "refresh_token": "jwt-refresh",
    "expires_in": 900
  },
  "memberships": [
    {
      "company_id": "c_001",
      "company_name": "Company X",
      "membership_id": "m_001"
    },
    {
      "company_id": "c_002",
      "company_name": "Company Y",
      "membership_id": "m_002"
    }
  ],
  "next_action": "select_company"
}
```

**Response — khong co company active**

HTTP `403` (hoac `422` tuy policy)

```json
{
  "error": {
    "code": "NO_ACTIVE_COMPANY_ACCESS",
    "message": "User does not have any active company membership."
  }
}
```

---

### POST /api/v1/auth/refresh

**Request**

```json
{
  "refresh_token": "jwt-refresh"
}
```

**Response**

```json
{
  "access_token": "new-access-token",
  "refresh_token": "new-refresh-token",
  "expires_in": 900,
  "current_context": {
    "company_id": "c_001",
    "membership_id": "m_001"
  }
}
```

Sau moi lan refresh thanh cong, client **phai** luu `refresh_token` moi; token cu khong con hop le (rotation).

Membership cua phien khong con `active` (bi khoa hoac bi xoa sau khi dang nhap) -> **401 `SESSION_EXPIRED`** (cap nhat 2026-10-10, H3); client xu ly nhu het phien.

---

### POST /api/v1/auth/logout

**Request**

```json
{
  "refresh_token": "jwt-refresh"
}
```

**Response**

```json
{
  "success": true
}
```

---

### Hệ thống: GET /healthz, GET /readyz

Public, **khong** can Bearer. Tra JSON `status` (xem `internal/httpserver`).

---

### Thiet bi phien nguoi dung (user sessions)

- **GET /api/v1/sessions** — can Bearer. Danh sach phien cua user.
- **POST /api/v1/sessions/{session_id}/revoke** — can Bearer, thu hoi phien theo `session_id`.

(Chi tiet truong JSON: dung cung mau hoa `snake_case` theo `iam/transport/http`.)

---

### POST /api/v1/me/active-company

**Alias cung hanh vi** nhu `POST /api/v1/auth/switch-company` (doi context cong ty khi dang dang nhap, Bearer access token theo cong ty cu). Dung cung `Authorization` + `{"company_id":"..."}`.

---

## B. Session / identity APIs

### GET /api/v1/me

**Response**

```json
{
  "user": {
    "user_id": "u_123",
    "login_id": "user_a",
    "full_name": "Nguyen Van A",
    "subscription_tier": "Premium",
    "subscription_expires_at": "2027-01-01T00:00:00Z"
  },
  "contact": {
    "email": "user@example.com",
    "phone": "+84901234567",
    "email_verified": true
  },
  "profile_schema_version": 1,
  "current_context": {
    "company_id": "c_001",
    "membership_id": "m_001"
  },
  "company_context": { "has_company": true, "active_company_id": "c_001", "companies": [] }
}
```

`subscription_expires_at` is `null` when the active tier row has no `effective_to`.

`contact.email` / `contact.phone` are profile contact fields (distinct from `login_id`).

**Alias:** `GET /api/v1/me/profile` cung phan hoi nhu tren.

### PATCH /api/v1/me/profile

Self-service profile update (Bearer access token; no `rbac.manage`). Body: optional `full_name`, `email`, `phone` (at least one field).

**Response 200:** `{ "ok": true }`

Reject when account is locked/suspended/disabled. Audit action `user.profile.update`.

### POST /api/v1/me/change-password

Same semantics as admin change-password (RSA cipher, min 12, revoke sessions). Request/response shape identical to `POST /api/v1/admin/account/change-password`.

### POST /api/v1/admin/account/change-password

**Request** (RSA required when `LOGIN_PASSWORD_RSA_PRIVATE_KEY_PEM` is set):

```json
{
  "current_password_cipher": { "alg": "RSA-OAEP-256", "kid": "default", "ciphertext_b64": "..." },
  "new_password_cipher": { "alg": "RSA-OAEP-256", "kid": "default", "ciphertext_b64": "..." }
}
```

**Response 200:** `{ "success": true }` — revokes all sessions for the user (re-login required).

---

### GET /api/v1/me/companies

**Response**

```json
{
  "items": [
    {
      "company_id": "c_001",
      "membership_id": "m_001",
      "company_name": "Company X",
      "membership_status": "active",
      "roles": ["admin_doanh_nghiep"],
      "titles": ["Giam doc Phap che"],
      "address": "123 Street, District 1"
    },
    {
      "company_id": "c_002",
      "membership_id": "m_002",
      "company_name": "Company Y",
      "membership_status": "active"
    }
  ]
}
```

---

## C. Company context APIs

### POST /api/v1/auth/select-company

Dung sau login khi `next_action` = `select_company`. Bat buoc ghi audit.

**Request**

```json
{
  "company_id": "c_001"
}
```

**Response**

```json
{
  "access_token": "company-bound-access-token",
  "expires_in": 900,
  "current_context": {
    "company_id": "c_001",
    "membership_id": "m_001"
  }
}
```

---

### POST /api/v1/auth/switch-company

Khi da dang nhap, doi context — phat hanh access token moi; khong dung token cu. Bat buoc ghi audit.

**Request**

```json
{
  "company_id": "c_002"
}
```

**Response**

```json
{
  "access_token": "new-company-bound-access-token",
  "expires_in": 900,
  "current_context": {
    "company_id": "c_002",
    "membership_id": "m_002"
  }
}
```

---

## C2. Company self-service provision

> Contract: `cobo_web_design/docs/contracts/company-self-service-create-nth.md`  
> Migration: `0082_companies_self_service_provisioning` (`founder_user_id`, `provisioning_source`)

### Feature flags (BE)

| Env | Default | Effect |
|-----|---------|--------|
| `COMPANY_SELF_CREATE_ENABLED` | `false` | When false, `POST /company/create` → **404** `FEATURE_DISABLED` |
| `COMPANY_PROVISION_IDEMPOTENCY_REQUIRED` | `false` | When true, `Idempotency-Key` header required on initialize/create |

### Feature flags (FE)

| Env | Default | Effect |
|-----|---------|--------|
| `VITE_COMPANY_SELF_CREATE_ENABLED` | unset/false | Hides Portal switcher CTA + profile “Tạo doanh nghiệp mới” |

### POST /api/v1/company/initialize

First company for user with **zero** eligible memberships (`active` + `invited`).

**Headers:** `Authorization: Bearer <access_token>`, optional/required `Idempotency-Key` (see flag).

**Request**

```json
{
  "company_name": "Demo Co",
  "tax_code": "0101234567",
  "registration_number": "",
  "address": "",
  "phone": "",
  "contact_email": ""
}
```

**Response 201**

```json
{
  "company_id": "c_uuid",
  "company_code": "co_xxx",
  "company_name": "Demo Co",
  "membership_id": "m_uuid",
  "session": {
    "access_token": "jwt-with-new-company",
    "refresh_token": "refresh",
    "expires_in": 3600
  }
}
```

**Errors:** `409` `COMPANY_ALREADY_EXISTS` (already has eligible membership), `403` `EMAIL_VERIFICATION_REQUIRED`, `409` `IDEMPOTENCY_CONFLICT`, `500` `SESSION_CONTEXT_UPDATE_FAILED`.

**Audit:** `company.initialize` — actor context uses **new** `company_id` / `membership_id`.

### POST /api/v1/company/create

Additional self-service company when user has **≥1** eligible membership and quota allows.

Same request/response shape as initialize. `provisioning_source = self_service_create`.

**Errors (additional):** `404` `FEATURE_DISABLED`, `402` `QUOTA_EXCEEDED` with `details: { "limit": 1, "current": 1, "tier": "Free" }`.

**Quota (self-provisioned count):** Free=1, Premium=3, Enterprise=unlimited (`limit` 0).

**Audit:** `company.create_self_service`.

**Idempotency scopes:** `company.initialize` | `company.create` (handler-level store, TTL 24h).

---

## D. Effective access APIs

### GET /api/v1/me/effective-access

**Response**

```json
{
  "company_id": "c_001",
  "membership_id": "m_001",
  "permissions": [
    "view_dashboard",
    "view_disclosure_obligation",
    "approve_disclosure"
  ],
  "data_scope": {
    "scope_type": "mixed",
    "departments": [
      {
        "department_id": "d_legal",
        "department_name": "Legal"
      },
      {
        "department_id": "d_ir",
        "department_name": "IR"
      }
    ],
    "record_assignments": [
      {
        "resource_type": "disclosure_record",
        "resource_id": "r_1001"
      }
    ],
    "has_company_wide_access": false
  },
  "responsibilities": [
    "notification_recipient:disclosure",
    "workflow_approver:disclosure",
    "direct_assignee"
  ]
}
```

---

### GET /api/v1/me/capabilities

**Response**

```json
{
  "modules": {
    "dashboard": true,
    "user_management": false,
    "department_management": false,
    "disclosure": true,
    "workflow_approval": true,
    "notification_config": false
  }
}
```

---

### GET /api/v1/me/membership

**Response**

```json
{
  "company_id": "c_001",
  "membership_id": "m_001",
  "roles": [
    "department_staff",
    "disclosure_approver"
  ],
  "departments": [
    "Legal",
    "IR"
  ],
  "titles": [
    "Dau moi CBTT"
  ]
}
```

---

## E. Internal authorization APIs

### POST /internal/v1/authorize

`subject` is **taken from the access token** (Bearer): `user_id`, `membership_id`, and `company_id` in the JSON body are **ignored** if present, so the decision always matches the caller’s JWT context. `action` and `resource` are unchanged. The authorization service (`Checker` / policy) is **not** altered—only the transport binding.

**Request** (illustrative; `subject` may be omitted; values below match token for documentation only)

```json
{
  "subject": {
    "user_id": "u_123",
    "membership_id": "m_001",
    "company_id": "c_001"
  },
  "action": "disclosure.approve",
  "resource": {
    "type": "disclosure_record",
    "id": "r_1001",
    "attributes": {
      "department_id": "d_legal",
      "status": "pending_approval"
    }
  }
}
```

**Response — allow**

```json
{
  "decision": "allow",
  "matched_permissions": [
    "approve_disclosure"
  ],
  "scope_reasons": [
    "department_membership:d_legal"
  ],
  "responsibility_reasons": [
    "workflow_assignee_rule:legal_approval"
  ],
  "deny_reason_code": null
}
```

**Response — deny**

```json
{
  "decision": "deny",
  "matched_permissions": [],
  "scope_reasons": [],
  "responsibility_reasons": [],
  "deny_reason_code": "COMPANY_SCOPE_MISMATCH"
}
```

---

### POST /internal/v1/authorize/batch

Same as single authorize: top-level `subject` is **derived from the access token**; body `subject` fields are ignored. `checks[]` unchanged.

**Request**

```json
{
  "subject": {
    "user_id": "u_123",
    "membership_id": "m_001",
    "company_id": "c_001"
  },
  "checks": [
    {
      "action": "disclosure.view",
      "resource": {
        "type": "disclosure_record",
        "id": "r_1001"
      }
    },
    {
      "action": "disclosure.approve",
      "resource": {
        "type": "disclosure_record",
        "id": "r_1001"
      }
    }
  ]
}
```

**Response (mau)**

```json
{
  "results": [
    {
      "decision": "allow",
      "matched_permissions": ["view_disclosure"],
      "scope_reasons": ["department_membership:d_legal"],
      "responsibility_reasons": [],
      "deny_reason_code": null
    },
    {
      "decision": "deny",
      "matched_permissions": [],
      "scope_reasons": [],
      "responsibility_reasons": [],
      "deny_reason_code": "PERMISSION_DENIED"
    }
  ]
}
```

---

## F. Access administration APIs (mau JSON toi thieu)

Cac API duoi day co the tra 201 Created / 200 OK tuy endpoint; y bat buoc: audit day du.

### POST /api/v1/admin/users

Tao tai khoan user truc tiep (admin flow). Endpoint nay tao ban ghi `users` + `credentials(password)`, va co the tao membership cung call neu gui `company_id`.

**Request**

```json
{
  "login_id": "new.user@example.com",
  "password": "StrongPass123!",
  "full_name": "New User",
  "email": "new.user@example.com",
  "phone": "0909123456",
  "account_status": "active",
  "company_id": "c_001",
  "membership_status": "active"
}
```

**Validation**

- `login_id`: required, unique (trim + lowercase)
- `password`: required, >= 8 chars
- `full_name`: required
- `account_status`: optional, default `active`
- `company_id`: optional. Neu co -> tao membership atomically trong cung request.
- `membership_status`: optional, default `active` khi co `company_id`.

**Authorization boundary**

- Route tenant `/api/v1/admin/*` luon thao tac tren company cua access token (cap nhat 2026-10-09, risk review C4):
  - `company_id` bo trong -> backend dung `current_context.company_id`. Route nay khong tao user "khong gan membership".
  - `company_id` khac company cua token -> **403 `COMPANY_SCOPE_MISMATCH`**, bat ke nguoi goi la ai.
- Thao tac cross-company hoac tao user khong gan company chi danh cho **platform operator** (`platform.cms.view` va (`rbac.manage` hoac `system.settings`)) qua `/api/v1/platform/cms/admin/*`.
  Quyen `rbac.manage` don le la quyen cua tenant, khong du de thao tac sang company khac.

**Response (201)**

```json
{
  "user_id": "u_new",
  "login_id": "new.user@example.com",
  "full_name": "New User",
  "email": "new.user@example.com",
  "phone": "0909123456",
  "account_status": "active",
  "membership_id": "m_new",
  "company_id": "c_001",
  "company_name": "Company One",
  "membership_status": "active"
}
```

---

### POST /api/v1/admin/memberships

`company_id` bo trong -> company cua token. Khac company cua token -> 403 `COMPANY_SCOPE_MISMATCH`.

**Request**

```json
{
  "company_id": "c_001",
  "user_id": "u_123",
  "membership_status": "active"
}
```

**Response**

```json
{
  "membership_id": "m_new",
  "company_id": "c_001",
  "user_id": "u_123",
  "membership_status": "active"
}
```

---

> **Phạm vi company (cập nhật 2026-10-09, risk review C5):** mọi route tenant `/api/v1/admin/**` nhận `membership_id` (path hoặc body) chỉ thao tác trên membership thuộc company của access token. Membership của company khác trả **404 `MEMBERSHIP_NOT_FOUND`** (giống membership không tồn tại, không lộ id có tồn tại hay không) và dữ liệu không bị thay đổi. Áp dụng cho cập nhật/xoá membership, gán/gỡ role, role chính, phòng ban, chức danh, org-assignments, quyền trực tiếp, thành viên team/title/department, company admin, transfer-ownership và config approval `rbac.direct_permission.remove`. Team, department, title của company khác trả **404** với `error.code` `INVALID_REQUEST` và message `team not found` / `department not found` / `title not found` (không còn 409 "có thành viên"). `POST /admin/company/admins` và `POST /admin/company/transfer-ownership` trả 404 `MEMBERSHIP_NOT_FOUND` (trước đây `INVALID_REQUEST`, status không đổi). `membership_id` rỗng vẫn là 400 `INVALID_REQUEST`. Token không có `company_id` trên các route này trả 422 `COMPANY_CONTEXT_REQUIRED`.

### PATCH /api/v1/admin/memberships/{membership_id}

**Request**

```json
{
  "status": "inactive"
}
```

Field request la `status` (handler khong doc `membership_status`). Chi nhan `active` hoac `inactive` (khong phan biet hoa thuong, da trim); gia tri khac -> **400 `INVALID_REQUEST`**. Khoa primary admin -> **409 `STATE_CONFLICT`** (`CANNOT_DEACTIVATE_PRIMARY_ADMIN`). Membership `inactive` mat toan bo quyen ngay lap tuc (cap nhat 2026-10-10, H3/BES-12).

**Response**

```json
{
  "membership_id": "m_001",
  "membership_status": "inactive"
}
```

---

### DELETE /api/v1/admin/memberships/{membership_id}

**Response**

```json
{
  "success": true
}
```

---

### GET /api/v1/admin/companies/{company_id}/memberships

`company_id` phai bang company cua token, nguoc lai 403 `COMPANY_SCOPE_MISMATCH`. Liet ke cross-company hoac "user khong co membership" chi qua `/api/v1/platform/cms/admin/users` (platform operator).

**Response**

```json
{
  "items": [
    {
      "membership_id": "m_001",
      "user_id": "u_123",
      "membership_status": "active"
    }
  ]
}
```

---

### POST /api/v1/admin/memberships/{membership_id}/roles

**Request**

```json
{
  "role_id": "r_role_staff"
}
```

**Response**

```json
{
  "membership_id": "m_001",
  "role_id": "r_role_staff",
  "status": "active"
}
```

---

### DELETE /api/v1/admin/memberships/{membership_id}/roles/{role_id}

Loi (cap nhat 2026-10-10):
- Role mang quyen platform (`platform.*`, `cms.*`), nguoi goi khong phai platform operator -> **403 `PERMISSION_DENIED`** (ROLE-13).
- Go role co `rbac.manage` cua primary admin -> **409 `STATE_CONFLICT`** (`CANNOT_REMOVE_PRIMARY_ADMIN_ROLE`).
- Go role khien cong ty khong con thanh vien admin-capable nao -> **409 `LAST_ADMIN_ROLE_CHANGE_BLOCKED`** (ROLE-05).

**Response**

```json
{
  "success": true
}
```

---

### POST /api/v1/admin/memberships/{membership_id}/departments

**Request**

```json
{
  "department_id": "d_legal",
  "effective_from": "2026-01-01T00:00:00Z",
  "effective_to": null
}
```

**Response**

```json
{
  "membership_id": "m_001",
  "department_id": "d_legal",
  "status": "active"
}
```

---

### DELETE /api/v1/admin/memberships/{membership_id}/departments/{department_id}

**Response**

```json
{
  "success": true
}
```

---

### POST /api/v1/admin/memberships/{membership_id}/titles

**Request**

```json
{
  "title_id": "t_head_cbtt"
}
```

**Response**

```json
{
  "membership_id": "m_001",
  "title_id": "t_head_cbtt",
  "status": "active"
}
```

---

### DELETE /api/v1/admin/memberships/{membership_id}/titles/{title_id}

**Response**

```json
{
  "success": true
}
```

---

### GET /api/v1/admin/permissions

**Response**

```json
{
  "items": [
    {
      "permission_id": "p_view_dashboard",
      "code": "view_dashboard",
      "description": "View dashboard"
    }
  ]
}
```

---

### GET /api/v1/admin/roles

**Response**

```json
{
  "items": [
    {
      "role_id": "r_staff",
      "code": "department_staff",
      "name": "Department staff"
    }
  ]
}
```

---

### POST /api/v1/admin/roles/{role_id}/permissions

**Request**

```json
{
  "permission_id": "p_approve_disclosure"
}
```

**Response**

```json
{
  "role_id": "r_approver",
  "permission_id": "p_approve_disclosure"
}
```

---

### DELETE /api/v1/admin/roles/{role_id}/permissions/{permission_id}

**Response**

```json
{
  "success": true
}
```

> **Phạm vi role (cập nhật 2026-10-09, risk review ROLE-01):** chỉ role `tenant_custom` của company trong token được sửa quyền. Role mặc định/dùng chung (`tenant_default`, `system_global`, `is_protected`) trả 403 `protected_role_read_only` (nhân bản thành role tùy chỉnh trước). Gỡ quyền critical vẫn đi qua approval (202 `APPROVAL_ROUTED`).

---

### POST /api/v1/admin/rbac/matrix/versions/{version_no}/rollback

Khôi phục ma trận RBAC của company về một phiên bản đã lưu. Body (tuỳ chọn): `{"reason": "..."}`.

> **Phạm vi (cập nhật 2026-10-09, risk review ROLE-01):**
> - **Role:** chỉ thay đổi quyền của role `tenant_custom` (không protected) của company trong token. Role `system_global`, `tenant_default`, protected và role của company khác **không bao giờ** bị thay đổi. Quyền ngoài phạm vi doanh nghiệp (module `cms`/`platform`, `platform.cms.view`, `cms.*`, ...) không bao giờ bị thêm hoặc gỡ.
> - **Thêm quyền** vào role chỉ khi `AssignRolePermission` cũng cho phép (grant tier `grantable` và cho role tùy chỉnh); quyền khác trong snapshot bị bỏ qua. Gỡ quyền thì luôn được phép.
> - **Quyền trực tiếp:** chỉ thu hồi/cấp lại các mã trong `GrantablePermissions` (mã admin tenant được quản lý qua `/memberships/{id}/permissions`) và chỉ cho membership của company. Quyền trực tiếp khác (ví dụ `platform.cms.view`, `ad_hoc_alert.process_control`) không bị đụng.
> - **Quyền hạn:** luôn cần `rbac.manage` (giống các route sửa quyền trực tiếp); chỉ có `system.settings` thì 403 `PERMISSION_DENIED` và không thay đổi gì (`system.settings` chỉ đủ để đọc phiên bản). Rollback thêm hoặc gỡ quyền trực tiếp `admin.membership.invite` chỉ primary admin được yêu cầu (giống route quyền trực tiếp), ngược lại 403.
> - **Approval:** nếu rollback thêm hoặc gỡ một quyền **critical**, thay đổi **chưa được áp dụng**: response **202** (body phẳng như các route approval khác, xem dưới); bản ghi `pending_admin_changes` có `change_type = "rbac.matrix.rollback"`. "Critical" gồm: `rbac.manage`, `system.settings`, `admin.membership.invite`, `disclosure.publish`, `disclosure.auto_create.manage`, `company.profile.manage` và mọi quyền có grant tier `tenant_admin_only` hoặc `high_risk`. Một thành viên **khác** người yêu cầu, cùng công ty, có `rbac.manage` hoặc `system.settings` duyệt qua `POST /api/v1/admin/config-approvals/{id}/approve` (xem mục "Quyền quyết định approval"); người yêu cầu không tự duyệt (403 `SELF_APPROVAL_NOT_ALLOWED`); ma trận đã có phiên bản mới thì 409 `STALE_PROPOSAL`. Công ty chỉ có một người quản trị không có đường tự duyệt: yêu cầu ở trạng thái `pending` tới khi có admin thứ hai hoặc người yêu cầu huỷ.
> - **Trong transaction:** nếu giữa lúc kiểm tra và lúc ghi có thay đổi khiến rollback chạm quyền critical, rollback tự chuyển sang approval (202), không áp dụng gì.
> - Rollback không có quyền critical áp dụng ngay và tạo phiên bản mới `source = "rollback"`.

**Response 200 (áp dụng ngay)**

```json
{
  "rolled_back_from": 3,
  "new_version": {
    "id": "…",
    "aggregate_type": "rbac_matrix",
    "version_no": 5,
    "source": "rollback"
  }
}
```

**Response 202 (cần approval)** — cùng dạng với `DELETE /api/v1/admin/roles/{role_id}/permissions/{permission_id}` khi được chuyển sang approval

```json
{
  "approval_id": "…",
  "status": "pending"
}
```

---

### Quyền quyết định approval (cập nhật 2026-10-09, ROLE-01 follow-up)

`POST /api/v1/admin/config-approvals/{id}/approve`, `/reject` và `/cancel` (khi người huỷ không phải người yêu cầu) cần **`rbac.manage` hoặc `system.settings`** (trước đây chỉ `system.settings`, mà không công ty nào có ai giữ quyền này). Luôn loại người yêu cầu (403 `SELF_APPROVAL_NOT_ALLOWED` khi duyệt/từ chối yêu cầu của chính mình). Người yêu cầu luôn huỷ được yêu cầu của mình. Đọc danh sách/chi tiết approval không đổi (`rbac.manage` hoặc `system.settings`).

**Approval RBAC không còn gì để áp dụng:** `POST .../approve` trả **409** `APPROVAL_NOTHING_TO_APPLY` (yêu cầu giữ `pending`; từ chối hoặc huỷ được) khi bản đề xuất không làm thay đổi gì mà thao tác khôi phục được phép thay đổi (ví dụ yêu cầu do phiên bản cũ tạo cho role mặc định/dùng chung, hoặc đã được áp dụng ngoài hàng đợi).

**`GET .../config-approvals/{id}/compare`** (aggregate `rbac_matrix`) trả thêm `changes`: danh sách thay đổi thật sẽ được áp dụng nếu duyệt (luôn có mặt, `[]` khi không còn gì để áp dụng; `null` với aggregate khác). `compare.changed_keys` không chứa các khoá nội bộ (`explicit`, `role_revokes`, `direct_revokes`, `plan_digest`):

```json
{
  "changes": [
    { "kind": "role_permission", "action": "remove", "role_id": "…", "role_code": "custom_x", "permission_code": "rbac.manage", "critical": true },
    { "kind": "direct_permission", "action": "add", "membership_id": "…", "permission_code": "admin.membership.invite", "critical": true }
  ]
}
```

**Approval áp dụng đúng điều đã duyệt:**
- `rbac.permission.remove` và `rbac.direct_permission.remove` là yêu cầu **một thay đổi**: khi duyệt chỉ thực hiện đúng việc gỡ được yêu cầu, không hội tụ cả ma trận về snapshot. Quyền trực tiếp cấp sau khi xếp hàng (mời người dùng, cấp mặc định) không bị thu hồi. Gỡ quyền trực tiếp áp dụng cho cả mã không thuộc danh sách tenant được cấp trực tiếp (giống route gỡ quyền trực tiếp). `rbac.direct_permission.remove` cho `admin.membership.invite` chỉ primary admin được yêu cầu.
- `rbac.matrix.rollback` hội tụ về snapshot nhưng gắn với **dấu vân tay kế hoạch** lúc xếp hàng: nếu lúc duyệt kế hoạch thực tế khác (ví dụ có quyền trực tiếp mới) thì **409 `STALE_PROPOSAL`** (kiểm cả trước và bên trong transaction), không áp dụng gì; hãy từ chối và yêu cầu lại.
- Tạo, nhân bản hoặc vô hiệu hoá role tùy chỉnh (`POST /roles`, `POST /roles/{id}/clone`, `DELETE /roles/{id}`) tạo phiên bản ma trận mới, nên mọi approval RBAC đang chờ trả 409 `STALE_PROPOSAL` khi duyệt.
- `notification_rule.patch` qua `POST /config-approvals` chỉ dành cho `alert_channel_prefs`, kiểm tra hợp lệ và gói cước giống `PATCH` thông thường (400 `INVALID_REQUEST` nếu sai); khi duyệt payload được kiểm tra lại.

### Route quản trị công ty cần `rbac.manage` (ROLE-04)

`POST /api/v1/admin/company/admins`, `DELETE /api/v1/admin/company/admins/{membership_id}` và `POST /api/v1/admin/company/transfer-ownership` cần `rbac.manage` (trước đây vô tình cần `system.settings`, nên chủ công ty tự đăng ký bị 403). Chỉ primary admin được chuyển quyền sở hữu; các kiểm tra khác không đổi. Người chỉ có `system.settings` nhận 403 `PERMISSION_DENIED`.

---

### Cấp role / quyền và các thay đổi hành vi khác (cập nhật 2026-10-10, risk review PR-A/PR-B)

- **Chỉ cấp được quyền mình có (ROLE-03).**
  - Áp dụng cho: `POST /api/v1/admin/users`, mời user (invite), `POST /api/v1/admin/memberships/{id}/roles`, `PUT .../primary-role`.
  - Người gọi không có `rbac.manage` và không phải platform operator chỉ được cấp role (và `permissions` trực tiếp khi mời) gồm các quyền chính họ đang có.
  - Ngược lại trả **403 `PERMISSION_DENIED`**, `error.details.permission_codes` = danh sách quyền còn thiếu.
  - `GET /api/v1/admin/invite-roles` chỉ trả các role người gọi được phép cấp theo quy tắc này.
- **`DELETE /api/v1/admin/memberships/{membership_id}/permissions/{permission_code}`** và config approval `rbac.direct_permission.remove`: gỡ quyền tầng platform (`platform.*`, `cms.*` và các mã trong `EnterpriseDenyCodes`, gồm `disclosure_type.config.*`) khi không phải platform operator → **403 `PERMISSION_DENIED`** (ROLE-13). Áp dụng cả khi duyệt một đề xuất như vậy đã xếp hàng (ROLE-19).
- **`PATCH /api/v1/admin/memberships/{membership_id}`** (`inactive`) và **`DELETE /api/v1/admin/memberships/{membership_id}`**: target đang giữ quyền tầng platform, người gọi không phải platform operator → **403 `PERMISSION_DENIED`** (BES-21).
- **`PUT /api/v1/admin/memberships/{membership_id}/primary-role`**: đổi primary admin sang role không có `rbac.manage` → **409 `STATE_CONFLICT`** (`CANNOT_REMOVE_PRIMARY_ADMIN_ROLE`); làm công ty mất admin cuối cùng → **409 `LAST_ADMIN_ROLE_CHANGE_BLOCKED`** (BES-19).
- **`POST /api/v1/admin/notification-rules/versions/{version_no}/rollback` (rule_id trong query/body) (ROLE-12)**, áp dụng cho rule `alert_channel_prefs`:
  - Rollback được đưa vào hàng chờ duyệt: **202** `{approval_id, status}` (như các route approval khác), `change_type` `notification_rule.patch`.
  - Snapshot đích vượt gói hiện tại → **402**.
  - Rule khác giữ nguyên 200.
- **`DELETE /api/v1/admin/notification-rules/{notification_rule_id}`**: rule `alert_channel_prefs` → **409 `STATE_CONFLICT`** (ROLE-12).
- **Duyệt approval `alert_channel_prefs`**: kiểm lại gói (tier của **người yêu cầu**, như lúc xếp hàng) tại thời điểm duyệt; vượt gói → **402** (BES-09/BES-20). Trên MySQL, apply giờ ghi đúng cột `payload_json` (trước đây mọi lần duyệt prefs trả 500, BES-18); rule đã bị xoá → 409 `STALE_PROPOSAL`.
- **Break-glass:**
  - Target của emergency grant không được duyệt chính grant đó → **403** (ROLE-14).
  - Membership inactive không nhận quyền từ overlay (BES-13).

### POST /api/v1/admin/config-approvals (change_type `rbac.permission.remove`)

Đề xuất gỡ quyền khỏi role qua approval. `role_id` phải là role `tenant_custom` của company (role protected hoặc dùng chung → 403 `protected_role_read_only`; role company khác → 404 `NOT_FOUND`) và `permission_id` phải thuộc phạm vi doanh nghiệp (ngược lại 400 `PERMISSION_OUT_OF_ENTERPRISE_SCOPE`).

---

### POST /api/v1/admin/resource-scope-rules

`company_id` trong body bi bo qua: rule luon thuoc company cua access token (cap nhat 2026-10-10, ROLE-02). Body rong -> 400 `INVALID_REQUEST`.

**Request**

```json
{
  "company_id": "c_001",
  "scope_type": "company_wide",
  "resource_type": "disclosure_record",
  "rule_json": {}
}
```

**Response**

```json
{
  "rule_id": "rsr_001",
  "company_id": "c_001"
}
```

---

### POST /api/v1/admin/workflow-assignee-rules

`company_id` trong body bi bo qua: rule luon thuoc company cua access token (cap nhat 2026-10-10, ROLE-02). Body rong -> 400 `INVALID_REQUEST`.

**Request**

```json
{
  "company_id": "c_001",
  "workflow_definition_id": "wf_disclosure_v1",
  "step_code": "legal_approval",
  "assignee_rule_json": {}
}
```

**Response**

```json
{
  "rule_id": "war_001"
}
```

---

### POST /api/v1/admin/notification-rules

**Request**

```json
{
  "company_id": "c_001",
  "event_type": "disclosure.submitted",
  "recipient_rule_json": {}
}
```

**Response**

```json
{
  "rule_id": "nr_001"
}
```

---

### PUT /api/v1/admin/disclosure-types/{type_id} — matrix `blocks` (six mandatory keys)

Khi body co mang `blocks` (it nhat mot phan tu), backend yeu cau **du 6 `block_key`** sau, **khong trung key**, `display_order` > 0 va **duy nhat** trong phien ban:

| `block_key` | Goi y noi dung (map cot phang legacy) |
|---|---|
| `legal_basis` | `legal_basis` |
| `disclosure_content` | `report_content` |
| `deadline` | `deadline_rule` |
| `channels_and_format` | `channels_text` + dong `Format: {format}` khi co `format` |
| `legal_risks` | `legal_risks_text` |
| `enterprise_workflow` | `implementation_content` |

Neu thieu bat ky key bat buoc, HTTP `400` + `error.details.field_errors` co khoa dang `blocks.missing_<block_key>`.

**Vi du payload toi thieu (custom + du 6 khoi):**

```json
{
  "group_id": "group-006",
  "name": "Mau noi bo",
  "template_category": "custom",
  "deadline_strategy": "configurable",
  "deadline_rule": "Theo quy dinh noi bo",
  "periodicity": "monthly",
  "legal_basis": "Quy che cong ty",
  "report_content": "Noi dung cong bo",
  "channels_text": "Website cong ty",
  "format": "PDF",
  "legal_risks_text": "Rui ro neu khong thuc hien",
  "implementation_content": "Quy trinh noi bo",
  "blocks": [
    {
      "block_id": "b1",
      "block_key": "legal_basis",
      "block_type": "rich_text",
      "title": "Co so phap ly",
      "description": "",
      "config": { "max_length": 8000, "allow_html": false },
      "validation": {},
      "display_order": 1,
      "enabled": true
    },
    {
      "block_id": "b2",
      "block_key": "disclosure_content",
      "block_type": "rich_text",
      "title": "Noi dung cong bo",
      "description": "",
      "config": { "max_length": 50000, "allow_html": true },
      "validation": {},
      "display_order": 2,
      "enabled": true
    },
    {
      "block_id": "b3",
      "block_key": "deadline",
      "block_type": "text",
      "title": "Ky han",
      "description": "",
      "config": { "max_length": 4000 },
      "validation": {},
      "display_order": 3,
      "enabled": true
    },
    {
      "block_id": "b4",
      "block_key": "channels_and_format",
      "block_type": "rich_text",
      "title": "Kenh va hinh thuc",
      "description": "",
      "config": { "max_length": 12000, "allow_html": false },
      "validation": {},
      "display_order": 4,
      "enabled": true
    },
    {
      "block_id": "b5",
      "block_key": "legal_risks",
      "block_type": "rich_text",
      "title": "Rui ro phap ly",
      "description": "",
      "config": { "max_length": 8000, "allow_html": false },
      "validation": {},
      "display_order": 5,
      "enabled": true
    },
    {
      "block_id": "b6",
      "block_key": "enterprise_workflow",
      "block_type": "rich_text",
      "title": "Workflow",
      "description": "",
      "config": { "max_length": 12000, "allow_html": true },
      "validation": {},
      "display_order": 6,
      "enabled": true
    }
  ]
}
```

---

### Company-level workflow override (tenant-isolated)

Muc tieu: admin doanh nghiep co the tuy chinh workflow rieng cho template, approve de ap dung cho chinh doanh nghiep do, khong anh huong template chung va doanh nghiep khac.

Base path: `/api/v1/company/disclosure-types/{type_id}/workflow-override`

Quyen goi y:
- `template.workflow.override.read`: GET override + GET versions + GET effective workflow
- `template.workflow.override.write`: PUT draft + DELETE draft version
- `template.workflow.override.approve`: POST approve
- `template.workflow.override.reset`: DELETE active (fallback ve global)

#### GET `/api/v1/company/disclosure-types/{type_id}/workflow-override`

Tra ve trang thai override hien tai (header + draft version + active version) va `effective_source`.

**Response 200**

```json
{
  "data": {
    "type_id": "dt-periodic-financial",
    "company_id": "c_001",
    "override": {
      "override_id": "ovr_c_001_dt-periodic-financial",
      "status": "approved",
      "active_version_no": 3,
      "updated_at": "2026-05-06T07:00:00Z"
    },
    "draft_version": {
      "version_no": 4,
      "state": "draft",
      "change_note": "Dieu chinh SLA buoc duyet",
      "workflow": [],
      "created_by": "u_001",
      "created_at": "2026-05-06T07:10:00Z"
    },
    "active_version": {
      "version_no": 3,
      "state": "approved",
      "change_note": "Go-live v3",
      "workflow": [],
      "created_by": "u_001",
      "approved_by": "u_002",
      "approved_at": "2026-05-05T10:30:00Z",
      "created_at": "2026-05-05T10:10:00Z"
    },
    "effective_source": "company_override"
  }
}
```

---

#### PUT `/api/v1/company/disclosure-types/{type_id}/workflow-override/draft`

Tao moi / cap nhat ban nhap workflow override cho company context tu token.

**Request**

```json
{
  "base_version_no": 3,
  "change_note": "Dieu chinh SLA buoc kiem duyet",
  "workflow": [
    {
      "step_id": "s1",
      "stage": "Chuan bi",
      "department": "Phap che",
      "assignee_role": "compliance_officer",
      "due_rule": "T+1",
      "display_order": 1,
      "documents": [
        {
          "doc_id": "d1",
          "name": "Bien ban doi soat",
          "required": true
        }
      ]
    }
  ]
}
```

**Response 200**

```json
{
  "override_id": "ovr_c_001_dt-periodic-financial",
  "type_id": "dt-periodic-financial",
  "company_id": "c_001",
  "draft_version_no": 4,
  "state": "draft",
  "updated_at": "2026-05-06T07:15:00Z"
}
```

---

#### POST `/api/v1/company/disclosure-types/{type_id}/workflow-override/approve`

Approve draft version de active cho doanh nghiep hien tai.

**Request**

```json
{
  "version_no": 4,
  "reason": "Da review voi team compliance"
}
```

**Response 200**

```json
{
  "override_id": "ovr_c_001_dt-periodic-financial",
  "type_id": "dt-periodic-financial",
  "company_id": "c_001",
  "active_version_no": 4,
  "state": "approved",
  "approved_by": "u_002",
  "approved_at": "2026-05-06T07:20:00Z",
  "effective_source": "company_override"
}
```

---

#### DELETE `/api/v1/company/disclosure-types/{type_id}/workflow-override/draft/{version_no}`

Xoa draft version theo `version_no`.

**Response 200**

```json
{
  "deleted": true,
  "version_no": 5
}
```

---

#### DELETE `/api/v1/company/disclosure-types/{type_id}/workflow-override/active`

Reset active override (fallback ve global template workflow).

**Request (optional)**

```json
{
  "reason": "Rollback ve workflow chuan"
}
```

**Response 200**

```json
{
  "override_id": "ovr_c_001_dt-periodic-financial",
  "type_id": "dt-periodic-financial",
  "company_id": "c_001",
  "active_version_no": 0,
  "state": "archived",
  "effective_source": "global_template"
}
```

---

#### GET `/api/v1/company/disclosure-types/{type_id}/workflow-override/versions?page=1&page_size=20`

Tra ve lich su versions cua company override.

**Response 200**

```json
{
  "items": [
    {
      "version_no": 4,
      "state": "approved",
      "change_note": "Dieu chinh SLA",
      "workflow": [],
      "created_by": "u_001",
      "approved_by": "u_002",
      "approved_at": "2026-05-06T07:20:00Z",
      "created_at": "2026-05-06T07:15:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "page_size": 20,
    "total": 1
  }
}
```

---

#### GET `/api/v1/disclosure-types/{type_id}/effective-workflow`

Endpoint consumer runtime de lay workflow hieu luc:

- neu co active approved override cua company -> source `company_override`
- neu khong -> source `global_template`

**Response 200**

```json
{
  "data": {
    "type_id": "dt-periodic-financial",
    "company_id": "c_001",
    "source": "company_override",
    "version_no": 4,
    "workflow": []
  }
}
```

---

#### Error matrix (workflow override)

| HTTP | `error.code` | Khi nao |
|---|---|---|
| 400 | `INVALID_REQUEST` | thieu/loi field bat buoc (`type_id`, `version_no`, `workflow`...) |
| 400 | `WORKFLOW_SCHEMA_INVALID` | schema workflow step/doc khong hop le |
| 401 | `SESSION_EXPIRED` | token het han/khong hop le |
| 403 | `PERMISSION_DENIED` | khong du quyen write/approve/reset |
| 403 | `COMPANY_SCOPE_MISMATCH` | truy cap vuot company scope |
| 404 | `TEMPLATE_NOT_FOUND` | `type_id` khong ton tai hoac khong thuoc scope |
| 404 | `OVERRIDE_NOT_FOUND` | chua co override ma thao tac approve/delete specific version |
| 409 | `STATE_CONFLICT` | approve nham version khong con `draft`, race condition |
| 422 | `MAKER_CHECKER_REQUIRED` | policy bat buoc nguoi approve khac nguoi tao (neu bat) |

`details.field_errors` duoc dung cho validate-level errors:

```json
{
  "error": {
    "code": "WORKFLOW_SCHEMA_INVALID",
    "message": "workflow payload is invalid",
    "details": {
      "field_errors": {
        "workflow[0].due_rule": "must match T+N or H+N",
        "workflow[1].documents[0].name": "is required"
      }
    }
  }
}
```

---

## G. HTTP status mapping (goi y)

| HTTP | Khi nao |
|---|---|
| 200 | Thanh cong, tra body |
| 201 | Tao moi thanh cong |
| 400 | Request khong hop le |
| 401 | Chua xac thuc / token loi |
| 403 | Khong du quyen policy / tenant |
| 404 | Khong tim thay resource |
| 409 | Trung lich / state conflict |
| 422 | Validate nghiep vu (vi du NO_ACTIVE_COMPANY_ACCESS) |

---

## Tai lieu lien quan

- `docs/implementation-step-by-step.md` — do uu tien trien khai va ma loi.
- `docs/ai-cache/cobo-iam-services-phase-a-overview-summary.md` — tom tat tong quan.
