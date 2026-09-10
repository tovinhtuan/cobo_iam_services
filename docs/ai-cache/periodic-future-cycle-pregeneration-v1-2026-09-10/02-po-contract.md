# PO contract (locked)

| Key | Value |
|-----|-------|
| FUTURE_CYCLE_PREGENERATION | true |
| PREGEN_ENTITY | periodic_cycle |
| CONFIG_FIELD | periodic_cycle_generation_lead_days |
| CONFIG_OWNER | TEMPLATE_VERSION |
| CONFIG_STORAGE | deadline_config_json |
| CONFIG_UNIT | CALENDAR_DAYS |
| CONFIG_MIN/MAX | 0 / 90 |
| NULL/ABSENT | CURRENT_SLOT_ONLY |
| ZERO | FUTURE_PREGEN_DISABLED |
| COMPANY_OVERRIDE | false |
| TARGET | NEXT_APPLICABLE_LOGICAL_SLOT_ONLY |
| GENERATE_AT | T_next − lead_days (HCM date-only) |
| PREGEN_ANCHOR | T |
| FULL_EARLY_MATERIALIZATION | false |
| DB_MIGRATION_REQUIRED | false |
| API_BREAKING_CHANGE | false |
