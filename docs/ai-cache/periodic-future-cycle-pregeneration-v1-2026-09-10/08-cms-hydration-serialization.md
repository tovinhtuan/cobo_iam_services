# Hydration / serialization

- cmsApi: read from deadline_config on detail + version hydrate
- templateDefaults: map periodicCycleGenerationLeadDays → form field (preserve 0)
- templateMappers: write numeric into deadline_config payload
- Empty input → null; 0 persists as 0
