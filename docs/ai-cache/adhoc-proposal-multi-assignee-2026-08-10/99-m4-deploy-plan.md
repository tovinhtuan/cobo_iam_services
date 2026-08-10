# Deploy plan

Order executed:

1. Apply 0128 ONLY via push-migration
2. `make deploy-be` (api + worker together; SCP linux binaries + force-recreate --no-deps api worker; also SCPs migrations dir — does not auto-apply)
3. healthz/readyz gate
4. `make deploy-fe` (web-only isolation verified by script)
5. Authenticated E2E

Nginx rate/burst not changed (20r/s burst 40).
