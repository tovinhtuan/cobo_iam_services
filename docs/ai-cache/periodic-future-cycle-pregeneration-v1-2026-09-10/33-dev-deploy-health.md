# DEV deploy + health

- Method BE: `make deploy-be` → PASS
- Method FE: `make deploy-fe` → PASS
- Host: 88.216.208.0:21239
- API_HEALTH=PASS (healthz/readyz)
- FE_HEALTH=PASS
- WORKER_HEALTH=PASS (Up; PERIODIC_SEEDING_ENABLED=true)
