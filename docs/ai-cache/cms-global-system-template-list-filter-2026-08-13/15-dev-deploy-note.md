# DEV deploy note

DEV verdict (FE evidence pack authoritative for UI): **CMS_GLOBAL_SYSTEM_TEMPLATE_LIST_DEV_READY**

- `make deploy-be` 2026-08-13 — API sha `bd53b384…`, worker restarted, `--no-deps` (no migrate apply)
- API smoke: scope=global/company/invalid + no-scope legacy PASS
- NO_MIGRATION / NO_PRODUCTION / NO_PUSH
