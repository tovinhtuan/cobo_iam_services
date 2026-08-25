# 07 — Worker logs (bounded)

```text
docker logs cobo-iam-worker --since 24h | egrep periodic|seed|bang-tinh-luong...
→ no SeedPeriodicCycles / materialization lines for this template
```

Expected when `PERIODIC_SEEDING_ENABLED=false`: ticker may still log other worker work (reminders etc.); disclosure seed path not wired.

No panic/fatal attributed to this template in the checked window.