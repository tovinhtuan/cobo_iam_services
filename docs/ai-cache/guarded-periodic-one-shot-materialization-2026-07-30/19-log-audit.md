# Log audit

- One-shot executed via host CLI (not API process) — API logs do not contain global seed.
- Flags: PERIODIC_SEEDING_ENABLED=false throughout.
- No panic observed during apply/verify.
- Shadow warn only: missing `configs/non_trading_days/2026.json` on host CLI cwd (WD weekend calc still produced 2026-07-31).
