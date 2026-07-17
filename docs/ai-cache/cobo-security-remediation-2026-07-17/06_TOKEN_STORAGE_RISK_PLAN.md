# Phase 6 — Token Storage Risk Plan

## Current risk
- Frontend stores access/refresh tokens in `localStorage`.
- If stored XSS occurs, attacker can exfiltrate tokens.

## Options
- **Option A (short-term):** keep localStorage + harden CSP/sanitization + strict rich HTML controls.
- **Option B (recommended mid-term):** access token in memory + refresh token in HttpOnly/Secure/SameSite cookie.
- **Option C (long-term):** BFF/session-cookie architecture.

## Recommendation
- Adopt **Option B** in staged rollout:
  1. backend cookie-capable refresh endpoint with CSRF protection
  2. frontend token handling migration feature-flagged
  3. fallback compatibility window
  4. remove localStorage refresh token.

## This cycle
- No auth-storage rewrite performed to avoid broad product behavior changes.
- Risk remains `Medium` with mitigation plan.
