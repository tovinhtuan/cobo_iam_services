# CMS template import — Báo cáo thường niên (2026-09-26)

## Verdict

```text
BROWSER_SESSION=PASS
STAGING_FILE=CLEANED
JSON_HASH_MATCH=PASS
BROWSER_UPLOAD=PASS
BROWSER_VALIDATE=PASS
REQUIRED_MAPPINGS=PRESENT
CMS_DRAFT_IMPORT=NOT_RUN
reason=VALIDATED_MAPPING_REQUIRED
PUBLISH=NOT_RUN
ACTIVATE=NOT_RUN
EMAIL_SENT=NOT_RUN
PRODUCTION_TOUCHED=false
FRONTEND_CHANGED=false
STAGING_CLEANUP=PASS
DEV_BROWSER_SMOKE=PASS
```

Validate succeeded. Draft was not created because the UI/API set `can_confirm=false` while three departments still need a tenant catalog mapping. No guessed mapping was selected.

## DEV call

- Screen: `http://88.216.208.0:3000/cms/templates/import`
- `POST /api/v1/platform/cms/templates/import/validate` => **200**
- `x-request-id`: `1e38adfc-96ac-4c87-bd7e-9abbca3a9d4e`
- Confirm endpoint was not called.
- Console errors after validate: 0
- Session APIs `/api/v1/me*` returned 200. Headers and tokens are omitted.

## Validation

| Field | Value |
|---|---|
| parse_valid | true |
| domain_valid | true |
| mapping_required | true |
| can_confirm | false |
| activation_ready | true |
| suggested_type_id | bao-cao-thuong-nien |
| preview periodicity | yearly |
| preview deadline_rule | T+110 |
| applicable_from_mode | NEXT_SLOT |
| domain errors | none |

Required mappings, not auto-matched:

- `corporate_secretary` — Thư ký Công ty / Văn phòng HĐQT
- `legal` — Phòng Pháp chế & Tuân thủ
- `bod` — Ban Giám đốc

Warnings are `UNRESOLVED_DEPARTMENT_MAPPING` only. No `INVALID_JSON_PAYLOAD`, `INVALID_PERIODICITY`, or `INVALID_APPLICABILITY_RULES`.

## Cleanup

Staging copy under `cobo_web_design/.codex-tmp/` was deleted. Source SHA-256 stayed `ACC52622E62BFD11C8B0AABA196F295B9DA59799A9BB3EC6E721986BF0E2EA00`. `cobo_web_design` git status is clean. Nothing was published or activated.
