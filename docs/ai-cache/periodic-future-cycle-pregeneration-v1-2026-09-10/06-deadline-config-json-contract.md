# deadline_config_json contract

```json
{
  "periodic_cycle_generation_lead_days": 20
}
```

- Integer only; string "20" rejected at CMS input (number input + Integer check)
- Legacy rows without key: parse as 0 / absent → CURRENT_SLOT_ONLY
- BE: TemplateDeadlineConfig.PeriodicCycleGenerationLeadDays *int
- ListActivePeriodicTypes: JSON_EXTRACT → PeriodicTypeRow.PeriodicCycleGenerationLeadDays int
