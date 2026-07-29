#!/bin/sh
# ensure-dev-fe-vite-flags.sh — DEV FE deploy only.
# Defaults required Vite flags to true, logs values, fails fast unless
# ALLOW_LEGACY_DEV_FLAGS=true (loud warning). Does not change app code.
#
# Usage (source so flags export into caller):
#   . ./scripts/ensure-dev-fe-vite-flags.sh
# Or execute for preflight-only validation:
#   sh scripts/ensure-dev-fe-vite-flags.sh
#   VITE_PERSONAL_OPS_V2=false sh scripts/ensure-dev-fe-vite-flags.sh  # expect exit 1
#
# Production / bare `make fe-build` must NOT call this script.

set -eu

export VITE_PERSONAL_OPS_V2="${VITE_PERSONAL_OPS_V2:-true}"
export VITE_DASHBOARD_OPERATIONAL_V2="${VITE_DASHBOARD_OPERATIONAL_V2:-true}"
export VITE_DASHBOARD_OVERVIEW_API_ENABLED="${VITE_DASHBOARD_OVERVIEW_API_ENABLED:-true}"
# Phase 12.7+: structured Legal Basis CMS editor (build-time).
export VITE_LEGAL_BASIS_STRUCTURED_CMS_ENABLED="${VITE_LEGAL_BASIS_STRUCTURED_CMS_ENABLED:-true}"

echo "[deploy-dev][fe] VITE_PERSONAL_OPS_V2=${VITE_PERSONAL_OPS_V2}"
echo "[deploy-dev][fe] VITE_DASHBOARD_OPERATIONAL_V2=${VITE_DASHBOARD_OPERATIONAL_V2}"
echo "[deploy-dev][fe] VITE_DASHBOARD_OVERVIEW_API_ENABLED=${VITE_DASHBOARD_OVERVIEW_API_ENABLED}"
echo "[deploy-dev][fe] VITE_LEGAL_BASIS_STRUCTURED_CMS_ENABLED=${VITE_LEGAL_BASIS_STRUCTURED_CMS_ENABLED}"

if [ "${ALLOW_LEGACY_DEV_FLAGS:-false}" = "true" ]; then
  echo "[deploy-dev][fe] WARNING: ALLOW_LEGACY_DEV_FLAGS=true — DEV may compile Legacy Profile / old dashboard paths."
  echo "[deploy-dev][fe] WARNING: This override is for emergency rollback only; do not use for normal DEV deploy."
  return 0 2>/dev/null || exit 0
fi

fail=0
[ "${VITE_PERSONAL_OPS_V2}" = "true" ] || {
  echo "ERROR: VITE_PERSONAL_OPS_V2 must be true for DEV FE deploy (got: '${VITE_PERSONAL_OPS_V2}')."
  echo "       Set ALLOW_LEGACY_DEV_FLAGS=true only for intentional legacy rollback."
  fail=1
}
[ "${VITE_DASHBOARD_OPERATIONAL_V2}" = "true" ] || {
  echo "ERROR: VITE_DASHBOARD_OPERATIONAL_V2 must be true for DEV FE deploy (got: '${VITE_DASHBOARD_OPERATIONAL_V2}')."
  fail=1
}
[ "${VITE_DASHBOARD_OVERVIEW_API_ENABLED}" = "true" ] || {
  echo "ERROR: VITE_DASHBOARD_OVERVIEW_API_ENABLED must be true for DEV FE deploy (got: '${VITE_DASHBOARD_OVERVIEW_API_ENABLED}')."
  fail=1
}
[ "${VITE_LEGAL_BASIS_STRUCTURED_CMS_ENABLED}" = "true" ] || {
  echo "ERROR: VITE_LEGAL_BASIS_STRUCTURED_CMS_ENABLED must be true for DEV FE deploy (got: '${VITE_LEGAL_BASIS_STRUCTURED_CMS_ENABLED}')."
  echo "       Set ALLOW_LEGACY_DEV_FLAGS=true only for intentional Legal Basis CMS textarea rollback."
  fail=1
}

if [ "$fail" -ne 0 ]; then
  return 1 2>/dev/null || exit 1
fi

echo "[deploy-dev][fe] Vite flag preflight OK"
return 0 2>/dev/null || exit 0
