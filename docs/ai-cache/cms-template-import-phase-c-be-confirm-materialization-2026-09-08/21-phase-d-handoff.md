# Phase D Handoff — CMS Template Import UI

## Scope for Phase D (Frontend Implementation)
Phase C backend APIs and Phase C.1 focused contract corrections are fully implemented, tested, and verified. The frontend can now implement the Import Upload & Confirmation UI under `src/features/cms-core/templates/import/`.

## Endpoints Summary
1. **Validate Upload**:
   - `POST /api/v1/platform/cms/templates/import/validate`
   - Content-Type: `multipart/form-data`
   - Form field: `file` (.json file, max 2 MiB)
   - Response: `200 OK`
     - `can_confirm`: boolean
     - `validation_token`: string (15 min TTL, actor-bound, payload-hash bound, purpose-bound `template_import`)
     - `preview`: object containing `normalized_template`, `validation_issues`, `reference_diffs`
2. **Confirm Import**:
   - `POST /api/v1/platform/cms/templates/import/confirm`
   - Content-Type: `application/json`
   - Body:
     ```json
     {
       "validation_token": "<token>",
       "target_type_id": "slug-id",
       "target_name": "<must match normalized_template.name>",
       "department_mappings": {
         "source_dept_id": "target_dept_code"
       },
       "normalized_template": { ... }
     }
     ```
   - Response: `201 Created`
     - `type_id`: string
     - `version_no`: 1
     - `is_active`: false
     - `is_released`: false
     - `portal_state`: "not_active"
     - `root_status`: "active"

## Error Contract & Status Matrix (Authoritative Lock Post-C.1)
- `400 Bad Request` (`INVALID_REQUEST`):
  - Malformed JSON body in Confirm request
  - Missing `validation_token`, `target_type_id`, or `target_name`
  - `target_name` mismatch against `normalized_template.name`
  - Unmapped department references or invalid display group codes
- `401 Unauthorized`: Missing or unauthenticated session
- `403 Forbidden` (`PERMISSION_DENIED`): Missing CMS platform permission `platform.cms.view` or `cms.template.write`
- `422 Unprocessable Entity` (`INVALID_IMPORT_TOKEN`):
  - Invalid validation token structure
  - Tampered HMAC signature
  - Expired validation token (> 15 minutes)
  - Token actor mismatch
  - Wrong token purpose
  - Wrong token schema version
  - Canonical payload hash mismatch
- `409 Conflict` (`STATE_CONFLICT`):
  - `target_type_id` collision (type already exists in database)

## Runtime & Security Prerequisites
- Mandatory environment variable: `CMS_TEMPLATE_IMPORT_SIGNING_SECRET` must be set in API server.
- Fail closed: If `CMS_TEMPLATE_IMPORT_SIGNING_SECRET` is unset, token issuance and verification fail closed (zero fallback to media signing secret).

## UI Behavior & Constraints
- Display validation warnings and blocker errors from Validate response.
- If there are unresolved department mappings (`reference_diffs.unmatched_departments`), present a dropdown mapping selector mapping each source department to an active target department (`department_code`).
- `target_name` field must match `preview.normalized_template.name` (cannot be changed independently).
- On 409 Conflict: notify user that `target_type_id` already exists and suggest choosing a different ID.
- On 422 Invalid Import Token: notify user that import session has expired or payload was altered; prompt to re-validate file.
- Token expires in 15 minutes; show countdown or re-upload prompt if expired.

