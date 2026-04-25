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
    "full_name": "Nguyen Van A"
  },
  "current_context": {
    "company_id": "c_001",
    "membership_id": "m_001"
  }
}
```

**Alias:** `GET /api/v1/me/profile` cung phan hoi nhu tren.

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
      "membership_status": "active"
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

- Admin doanh nghiep: chi duoc tao nhan vien thuoc company hien tai; backend se ep `company_id = current_context.company_id`.
- Admin web (co quyen `rbac.manage`): co the tao user bat ky, bao gom cross-company hoac tao user khong gan membership ngay.

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

### PATCH /api/v1/admin/memberships/{membership_id}

**Request**

```json
{
  "membership_status": "inactive"
}
```

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

---

### POST /api/v1/admin/resource-scope-rules

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
