---
name: api-postman-sync
description: Dùng khi thêm/sửa endpoint Cobo cần cập nhật tài liệu contract và Postman - postman/by-screen collections, Cobo_Local environment, regenerate_ai_cache_collection.mjs, docs/api-contracts-json.md, api-v1-implemented-contracts.json, openapi snapshot; không lưu token thật trong collection.
---

# API ↔ Postman / Contract Docs Sync

## Where things live (verify)
- Web repo `postman/`: `by-screen/NN_<Area>/` collections (01_Web_Auth …
  16_CMS_Taxonomy_Settings_Contract), environment
  `Cobo_Local.postman_environment.json` (`baseUrl`, `accessToken`,
  `refreshToken`, `preCompanyToken`, `companyId`, entity IDs), `IMPORT.txt`,
  `examples/disclosure-templates/*.put-body.json`,
  `regenerate_ai_cache_collection.mjs` → `docs/ai-cache/cobo-portal-postman-collection.json`.
- IAM repo `docs/`: `api-contracts-json.md`,
  `api-v1-implemented-contracts.json`, `openapi/v1-iam-snapshot.yaml`,
  Postman collections.

## Workflow
1. After the contract is final (`backend-api-contract` [iam]), add/update the
   request in the matching `by-screen` collection: method, path with
   `{{baseUrl}}`, auth `Bearer {{accessToken}}`, example body, and example
   responses (success + main error codes).
2. Use environment variables for IDs and tokens; set tokens via test scripts
   from login responses, never paste real tokens.
3. Update backend contract docs (JSON + markdown + OpenAPI snapshot) in the
   same change.
4. Regenerate the merged collection:
   ```bash
   cd cobo_web_design && node postman/regenerate_ai_cache_collection.mjs
   ```
5. Scan for secrets before committing:
   ```bash
   python3 .claude/skills/secrets-cors-cookie-review/scripts/scan_secrets.py postman docs/ai-cache/cobo-portal-postman-collection.json
   ```
6. Optional run with newman (only if installed; local env only).

## Guardrails
- Environment files committed with empty token values.
- Don't rename existing requests used by QA without noting it.

## Output format
- Endpoints added/updated
- Collections and docs changed
- Regeneration result
