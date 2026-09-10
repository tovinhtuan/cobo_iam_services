# DEV CMS config proof

Template: qa-resmoke-periodic-20260904a
API PUT/GET `/api/v1/admin/disclosure-types/{id}/config`
- Save lead_days=25 → 200
- Reload → 25
DEV_CMS_CONFIG_SAVE=PASS
DEV_CMS_CONFIG_RELOAD=PASS
DEV_CMS_CONFIG_VALUE=25
Note: required temporary role_permissions grant platform.cms.* on admin_web for API access (DEV QA)
