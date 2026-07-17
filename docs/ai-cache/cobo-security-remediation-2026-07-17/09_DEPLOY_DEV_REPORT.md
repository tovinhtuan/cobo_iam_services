# Phase 9 — Deploy DEV Report

## Deployment method
- Existing standard method from repo: `make deploy-be`.
- Target: `root@88.216.208.0:/root/cobo_project`.
- Recreated containers: `api`, `worker`.

## Pre-deploy secret hardening
- Added masked runtime secrets in server `.env`:
  - `INTERNAL_REMINDER_TOKEN`
  - `CMS_MEDIA_UPLOAD_SIGNING_SECRET`

## Post-deploy status
- `healthz`: `{"status":"ok"}`
- `readyz`: `{"status":"ready"}`
- FE entrypoint still reachable: `http://88.216.208.0:3000`.
