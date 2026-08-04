# Phase 8 — Source lineage

Exact commits from `git log` on `recovery/lost-changes-audit-20260717-153324`.

## Backend (`cobo_iam_services`)

| Phase | Commit | Message |
|-------|--------|---------|
| 0 | `6d36a07` | docs(subscription): lock company premium implementation contract |
| 1 domain | `11b2fd4` | feat(subscription): add company plan domain foundation |
| 1 schema | `2d2f185` | feat(subscription): add company subscription schema |
| 1 evidence | `2e93759` | docs(subscription): add company plan phase 1 foundation evidence |
| 2 | `a1a4db2` | feat(subscription): harden company plan reader and DEV seed |
| 2 evidence | `aa840fe` | docs(subscription): record company plan phase 2 evidence |
| 3 | `4a42fff` | feat(subscription): expose additive company plan on own company APIs |
| 3 evidence | `a8295e3` | docs(subscription): record company plan phase 3 API exposure evidence |
| 4 fix | `439207a` | fix(subscription): resolve company plan before PatchOwnCompany mutation |
| 4 evidence | `dd0ff1e` | docs(subscription): record company plan phase 4 quality evidence |
| 5 evidence | `42e3fc5` | docs(subscription): add company plan backend dev evidence |
| 6 pointer | `2c86906` | docs(subscription): mark phase 6 frontend consumer ready |
| 7 pointer | `1f48c27` | docs(subscription): mark phase 7 frontend dev ready |
| 7.1 nginx | `6d93e75` | fix(web): raise FE API proxy rate limit for company switch |
| **HEAD** | `6d93e75` | (tip) |

**Runtime API binary** was built/deployed from source through **`dd0ff1e`** (Phase 5 `MODE=be`). Later commits are docs and nginx web-proxy only — they do **not** change the running API binary.

## Frontend (`cobo_web_design`)

| Phase | Commit | Message |
|-------|--------|---------|
| Verified-email A | `d8e5728` | fix(portal): compact verified email status |
| 6 source | `1eeed38` | feat(portal): show premium by company subscription |
| 7 evidence | `378725a` | docs(portal): add company premium frontend dev evidence |
| 7.1 evidence | `bdccb81` | docs(portal): revalidate company switch after nginx rate-limit fix |
| **HEAD** | `bdccb81` | (tip) |

**Runtime FE JS** deployed from **`1eeed38`** via `make deploy-fe` → asset `index-BgDmCxEY.js`.
