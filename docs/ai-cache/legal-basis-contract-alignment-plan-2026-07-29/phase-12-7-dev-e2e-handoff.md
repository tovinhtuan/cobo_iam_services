# Phase 12.7 — DEV E2E handoff

## Verdict

**PASS_DEV_E2E_WITH_NON_BLOCKING_GAPS**

## What was proven on DEV (`88.216.208.0`)

1. BE `c6dd866` + FE `73f4642` deployed with isolation (MySQL unchanged; FE did not recreate API).
2. Flags: structured CMS/write **ON**, legacy fallback **ON**, require-on-publish **OFF**.
3. CMS structured editor created 2 Legal Basis items, reordered (B then A), saved `legal_bases[]` (no `clientId`).
4. Backend persisted `legal_bases_json` (count=2), projection OD-7 matched.
5. CMS reload (edit + Nội dung Portal) shows codes/titles preserved.
6. Tenant `#phap-ly` renders 2 cards; tenant GET detail returns `legal_bases` length 2.
7. Legacy `dt-periodic-financial` still renders fallback card; record not mutated.
8. No migration, no backfill apply, no direct DB writes, no production.

## Non-blocking gaps

- Activate blocked by `TEMPLATE_NO_WORKFLOW` (needs enterprise workflow) — not a Legal Basis persistence failure.
- New-version UI path N/A for this entity.
- Backfill execution remains **DEFERRED**.

## Stop

Do **not** open Phase 12.8 automatically. Await user confirmation.

## Accounts used (emails only)

- CMS/platform: `platform.tenant.admin@example.com`
- Tenant: `tvttthptlvh@gmail.com`
