# Phase 0 — Approval gate

**Date:** 2026-08-04  
**Mode:** CONTRACT LOCK ONLY — no source, migration, or deploy

## Historical evidence (pre-prompt)

| Source | Case C status |
|--------|----------------|
| `company-premium-backend-implementation-plan-2026-08-03/20-open-decisions.md` #1 | **Open / blocker** — Prefer C; needs Product+Backend |
| `21-plan-handoff.md` | Await Product/Backend/Security before coding |
| `09-recommended-api-contract.md` | `RECOMMENDED_PENDING_APPROVAL` (Case B interim fields still present) |
| `results.json` plan pack | SoT recommended C long-term; **not** signed approval |

**Conclusion from history alone:** Case C was **not** approved. Recommendation ≠ approval.

## Current user instruction (2026-08-04)

User execution prompt explicitly directs:

> CASE C — Company paid-plan source độc lập  
> `company_subscriptions` là source-of-truth  
> không lấy từ `user.subscriptionTier`  
> không dùng `CompanyTierResolver` làm billing SoT  

Allowed approval channel per prompt: **user instruction**.

## Gate checklist (10 items)

| # | Requirement | Status | Basis |
|---|-------------|--------|-------|
| 1 | CASE C independent paid-plan SoT | **APPROVED** | User instruction 2026-08-04 |
| 2 | `company_subscriptions` is SoT | **APPROVED** | User instruction + plan §12 |
| 3 | No paid plan → `plan: null` | **APPROVED** | Phase 0 contract §5.4 |
| 4 | Additive API `plan` | **APPROVED** | User instruction §8 |
| 5 | Shared `companyplan.Reader` | **APPROVED** | User instruction §7 |
| 6 | Expose GetOwnCompany + `/me/companies` | **APPROVED** | User instruction §8 |
| 7 | Member-scoped authz (`company.view` / membership) | **APPROVED** | User instruction §8.3 + prior security review |
| 8 | No billing-sensitive fields | **APPROVED** | User instruction locked safety |
| 9 | FE no fallback to user tier | **APPROVED** | User instruction fail-closed |
| 10 | Remove personal Premium only after BE DEV smoke | **APPROVED** | User instruction Phase 6/7 order |

## Explicitly NOT approved (remain deferred)

| Topic | Rule until further Product note |
|-------|----------------------------------|
| TRIAL shows Premium badge | **No** — badge only when `status == ACTIVE` |
| ENTERPRISE badge on company overview | **Deferred** — Phase 0 badge rule is `PREMIUM` + `ACTIVE` only |
| Platform CMS company detail `plan` | **Deferred** — Security soft open from prior pack |
| Admin write/PATCH company plan API | **Out of scope** for early phases (read path first; seed/fixture for DEV) |

## Gate verdict

**`PHASE_0_CONTRACT_LOCKED`**

Case C is approved via **user instruction** elevating prior recommendation.  
Historical docs alone would have been `BLOCKED_PENDING_CASE_C_APPROVAL`.

No Backend/Frontend runtime source written in Phase 0.
