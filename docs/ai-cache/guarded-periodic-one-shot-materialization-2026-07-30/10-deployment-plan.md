# Deployment plan

1. Build linux amd64 CLI locally
2. SCP to `/root/cobo_project/bin/periodic-materialize-one`
3. Do NOT recreate API/worker/MySQL/FE
4. Run preview → apply with confirm token
5. Verify DB/API/UI
