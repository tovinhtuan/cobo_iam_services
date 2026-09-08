# Confirm API Handler Implementation

## Route & Security
- **Route**: `POST /api/v1/platform/cms/templates/import/confirm`
- **Content-Type**: `application/json`
- **Gate**: Platform CMS template write permission (`platform.cms.view` + `cms.template.write`)
- **Authentication**: `Bearer <access_token>` inspected via `h.inspector.InspectAccessToken`

## Pipeline in Handler
1. Subject extraction from bearer access token:
   - Returns 401 if token is missing or invalid.
2. JSON body decoding into `ConfirmTemplateImportRequest`:
   - Returns 400 Bad Request if body is malformed.
3. Injection of authenticated `Subject` into request DTO:
   - Client-provided `Subject` or actor is disregarded.
4. Invocation of `s.svc.ConfirmTemplateImport(r.Context(), req)`:
   - On error: translated to standard API error via `httpx.WriteError(w, nil, err)`.
5. Post-Commit Best-Effort Audit Logging:
   - Action: `disclosure.type.import`
   - ResourceType: `disclosure_type`
   - ResourceID: `resp.TypeID`
   - Metadata: `creation_mode: "TEMPLATE_IMPORT"`, `target_type_id`, `version_no: 1`, `schema_version: "1.0"`, `payload_hash`, `actor_id`.
   - Never logs: raw validation token, HMAC secret, auth token, or full imported template JSON.
6. Returns HTTP 201 Created with JSON response:
   - `type_id`: Target type identifier
   - `version_no`: 1
   - `is_active`: false
   - `is_released`: false
   - `portal_state`: "not_active"
   - `root_status`: "active"
   - `created_at`: ActivatedAt timestamp string (empty/null for draft)
