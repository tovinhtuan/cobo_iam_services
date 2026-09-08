# Phase C.1 Evidence — 00 Context & Objective

## Task Overview
- **Mode**: IMPLEMENTATION MODE — FOCUSED CORRECTION ONLY
- **Subject**: COBO CMS — IMPORT TEMPLATE — PHASE C.1
- **Scope**: Delta-only correction pass over Phase C.
- **Goals**:
  - Close P1-01: ApplicabilityRules fidelity
  - Close P1-02: Import token HTTP contract drift
  - Close P1-03: Full importable field roundtrip proof
  - Close P1-04: Signing secret fallback contract hardening
  - Close GAP-01: MySQL concurrency / rollback proof
- **Hard Constraints**:
  - `OPEN_P0 = 0`, `OPEN_P1 = 0`
  - Zero FE source changes (`FE_SOURCE_CHANGED = false`)
  - Zero DB migrations introduced
  - Zero DEV deployment
  - Preserved Phase C baseline: `UpsertTypeVersion(CreateOnly=true)`, stateless HMAC tokens, best-effort post-commit audit
