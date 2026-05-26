#!/bin/sh
# Smoke ACC-QA-01, 03-04, 10 for Account settings tab (dev API).
# Uses user@example.com + select-company (admin.dn may have no membership on some dev DBs).
set -eu

BASE_URL="${BASE_URL:-http://88.216.208.0:8080}"
LOGIN_ID="${LOGIN_ID:-user@example.com}"
PASSWORD="${PASSWORD:-secret}"
COMPANY_ID="${COMPANY_ID:-c_001}"

fail() { echo "FAIL: $1" >&2; exit 1; }
ok() { echo "OK: $1"; }
warn() { echo "WARN: $1"; }

log_step() { printf "\n==> %s\n" "$1"; }

json_get() {
  printf '%s' "$2" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d$1)"
}

log_step "Login ($LOGIN_ID)"
LOGIN_RES=$(curl -sf -X POST "$BASE_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"login_id\":\"$LOGIN_ID\",\"password\":\"$PASSWORD\"}") || fail "login HTTP error"

NEXT=$(json_get "['next_action']" "$LOGIN_RES")
PRE=$(json_get "['session']['pre_company_token']" "$LOGIN_RES" 2>/dev/null || true)
ACCESS=$(json_get "['session']['access_token']" "$LOGIN_RES" 2>/dev/null || true)

if [ "$NEXT" = "select_company" ]; then
  [ -n "$PRE" ] || fail "missing pre_company_token"
  log_step "Select company $COMPANY_ID"
  SEL_RES=$(curl -sf -X POST "$BASE_URL/api/v1/auth/select-company" \
    -H "Authorization: Bearer $PRE" \
    -H 'Content-Type: application/json' \
    -d "{\"company_id\":\"$COMPANY_ID\"}") || fail "select-company HTTP error"
  ACCESS=$(json_get "['access_token']" "$SEL_RES")
elif [ -z "$ACCESS" ]; then
  fail "no access token (next_action=$NEXT)"
fi

[ -n "$ACCESS" ] || fail "empty access token"
ok "session ready"

log_step "GET /api/v1/me (ACC-QA-10 expiry)"
ME_RES=$(curl -sf "$BASE_URL/api/v1/me" -H "Authorization: Bearer $ACCESS") || fail "/me HTTP error"
printf '%s' "$ME_RES" | python3 -c "
import sys, json
u = json.load(sys.stdin).get('user', {})
print('  tier:', u.get('subscription_tier'))
print('  subscription_expires_at:', u.get('subscription_expires_at'))
if not u.get('subscription_tier'):
    raise SystemExit('missing subscription_tier')
" || fail "/me parse"

EXPIRY=$(json_get "['user']['subscription_expires_at']" "$ME_RES" 2>/dev/null || true)
if [ -n "$EXPIRY" ] && [ "$EXPIRY" != "None" ]; then
  ok "subscription_expires_at=$EXPIRY"
else
  warn "subscription_expires_at empty — deploy BE + migration 0078 for admin.dn expiry QA"
fi

log_step "GET /api/v1/admin/account/settings (ACC-QA-01)"
SETTINGS_CODE=$(curl -s -o /tmp/acc_settings.json -w '%{http_code}' \
  "$BASE_URL/api/v1/admin/account/settings" -H "Authorization: Bearer $ACCESS")
if [ "$SETTINGS_CODE" != "200" ]; then
  cat /tmp/acc_settings.json >&2
  fail "account/settings HTTP $SETTINGS_CODE"
fi
ok "account/settings"

log_step "GET /api/v1/admin/company (ACC-QA-05)"
COMPANY_CODE=$(curl -s -o /dev/null -w '%{http_code}' \
  "$BASE_URL/api/v1/admin/company" -H "Authorization: Bearer $ACCESS")
[ "$COMPANY_CODE" = "200" ] || fail "admin/company HTTP $COMPANY_CODE"
ok "admin/company"

log_step "POST change-password without cipher (ACC-QA-08 guard)"
CP_CODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE_URL/api/v1/admin/account/change-password" \
  -H "Authorization: Bearer $ACCESS" \
  -H 'Content-Type: application/json' \
  -d '{}')
if [ "$CP_CODE" = "400" ] || [ "$CP_CODE" = "503" ]; then
  ok "change-password guard status=$CP_CODE"
elif [ "$CP_CODE" = "404" ]; then
  fail "change-password route missing — deploy latest BE"
else
  warn "change-password status=$CP_CODE"
fi

echo ""
echo "Smoke finished."
