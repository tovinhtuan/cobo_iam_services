#!/bin/sh
# smoke-dev-fe-profile-v2.sh — Post-deploy check: DEV FE bundle includes Personal Ops V2.
# Detects Legacy Profile regression from missing VITE_PERSONAL_OPS_V2 at build time.
#
# Usage:
#   sh scripts/smoke-dev-fe-profile-v2.sh
#   FE_BASE_URL=http://88.216.208.0:3000 sh scripts/smoke-dev-fe-profile-v2.sh
#
# Exit 0 = personal-ops markers present in live JS
# Exit 1 = markers missing (likely legacy profile build)

set -eu

FE_BASE_URL="${FE_BASE_URL:-http://88.216.208.0:3000}"
FE_BASE_URL="${FE_BASE_URL%/}"

echo "[smoke-dev-fe] Checking Personal Ops V2 markers at ${FE_BASE_URL}"

html="$(curl -fsS "${FE_BASE_URL}/")" || {
  echo "ERROR: cannot fetch ${FE_BASE_URL}/"
  exit 1
}

js_path="$(printf '%s' "$html" | grep -oE '/?assets/index-[^"[:space:]]+\.js' | head -1 || true)"
if [ -z "$js_path" ]; then
  echo "ERROR: no assets/index-*.js found in index.html"
  exit 1
fi
case "$js_path" in
  /*) js_url="${FE_BASE_URL}${js_path}" ;;
  *)  js_url="${FE_BASE_URL}/${js_path}" ;;
esac

echo "[smoke-dev-fe] JS asset: ${js_url}"

js="$(curl -fsS "$js_url")" || {
  echo "ERROR: cannot fetch ${js_url}"
  exit 1
}

# Markers from personal-ops V2 layout (stable data-testid / class fragments)
if printf '%s' "$js" | grep -q 'personal-ops-root'; then
  echo "[smoke-dev-fe] PASS: found personal-ops-root (Personal Ops V2)"
  exit 0
fi

if printf '%s' "$js" | grep -q 'personal-ops-header-kpi'; then
  echo "[smoke-dev-fe] PASS: found personal-ops-header-kpi (Personal Ops V2)"
  exit 0
fi

echo "ERROR: Personal Ops V2 markers missing in live bundle."
echo "       Likely VITE_PERSONAL_OPS_V2 was not true at FE build — Profile may be Legacy."
echo "       Redeploy with: make deploy-fe   (or sh deploy-dev.sh fe)"
exit 1
