# 11 — Secret / generated file review

## Secret scan

```text
SECRET_SCAN=PASS
```

Notes:
- Phase 3 `run-api-verify.py` / `run-browser-e2e.mjs` use default DEV fixture password `secret` via env override — **exclude from clean commit candidate** (same class as other QA smoke scripts; do not treat as production secret leak in evidence markdown).
- No live Bearer tokens committed in evidence JSON.

## Generated / exclude

```text
deploy-artifacts/backend/bin/*
deploy-artifacts/web/dist/*
screenshots (optional; exclude unless policy requires)
_tmp_dav1_browser.mjs on FE
```

```text
GENERATED_ARTIFACT_REVIEW=PASS (classified; excluded from candidate)
```
