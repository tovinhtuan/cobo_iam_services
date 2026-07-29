#!/bin/sh
# Static isolation gate for make deploy-fe (Phase 11).
# Fails if expanded dry-run or web-perms helper would touch API / full stack.
set -eu

ROOT="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail=0
note() { printf '%s\n' "$*"; }
err() { printf 'FAIL: %s\n' "$*" >&2; fail=1; }

note "[isolation] Checking make -n deploy-fe + fix-dev-web-perms.sh"

DRY="$(make -n deploy-fe 2>/dev/null || true)"
HELPER="$ROOT/scripts/fix-dev-web-perms.sh"

echo "$DRY" | grep -Eiq 'compose[[:space:]]+down' && err "dry-run contains docker compose down" || true
echo "$DRY" | grep -Eiq 'restart[[:space:]]+api|compose[[:space:]].*restart.*api' && err "dry-run restarts api" || true

# Helper must not force-recreate api or up api+web together.
if grep -nE 'force-recreate[[:space:]]+.*api|force-recreate[[:space:]]+api' "$HELPER" >/dev/null 2>&1; then
  err "fix-dev-web-perms.sh still force-recreates api"
fi
if grep -nE 'up -d --force-recreate api web|up -d api web|up -d --force-recreate web api' "$HELPER" >/dev/null 2>&1; then
  err "fix-dev-web-perms.sh targets api+web together"
fi
if ! grep -nE 'up -d --no-deps web' "$HELPER" >/dev/null 2>&1; then
  err "fix-dev-web-perms.sh missing 'up -d --no-deps web'"
fi

# dry-run should invoke the helper (web path) and only stop web, not stop api
echo "$DRY" | grep -Eq 'stop web' || err "dry-run should stop web only (expected 'stop web')"
echo "$DRY" | grep -Eiq 'stop api' && err "dry-run must not stop api" || true
echo "$DRY" | grep -Fq 'fix-dev-web-perms.sh' || err "dry-run missing fix-dev-web-perms.sh"

if [ "$fail" -ne 0 ]; then
  note "[isolation] FAILED"
  exit 1
fi

note "[isolation] PASS: deploy-fe chain is web-only (no api recreate)"
exit 0
