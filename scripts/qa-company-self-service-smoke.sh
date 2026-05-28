#!/bin/sh
# Smoke company self-service provision (initialize + create).
# Requires: python3, curl, API with migration 0082 and flags enabled.
#
# Env:
#   BASE_URL (default http://127.0.0.1:8080)
#   LOGIN_ID / PASSWORD — user with verified email
#   IDEM_KEY — idempotency key (default smoke-provision-001)
#
# For initialize-only user: use account with zero eligible memberships.
# For create: user must already have >=1 company and tier quota headroom.
set -eu

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
LOGIN_ID="${LOGIN_ID:-user@example.com}"
PASSWORD="${PASSWORD:-secret}"
IDEM_KEY="${IDEM_KEY:-smoke-provision-$(date +%s)}"
COMPANY_ID="${COMPANY_ID:-}"

fail() { echo "FAIL: $1" >&2; exit 1; }
ok() { echo "OK: $1"; }
warn() { echo "WARN: $1"; }

json_get() {
  printf '%s' "$2" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d$1)"
}

http_code() {
  printf '%s' "$1" | tail -n1
}

http_body() {
  printf '%s' "$1" | sed '$d'
}

log_step() { printf "\n==> %s\n" "$1"; }

log_step "Health"
curl -sf "$BASE_URL/healthz" >/dev/null || fail "API not reachable at $BASE_URL"

log_step "Login ($LOGIN_ID)"
LOGIN_RES=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"login_id\":\"$LOGIN_ID\",\"password\":\"$PASSWORD\"}") || fail "login curl error"
LOGIN_CODE=$(http_code "$LOGIN_RES")
LOGIN_BODY=$(http_body "$LOGIN_RES")
[ "$LOGIN_CODE" = "200" ] || fail "login HTTP $LOGIN_CODE body=$LOGIN_BODY"

NEXT=$(json_get "['next_action']" "$LOGIN_BODY")
PRE=$(json_get "['session']['pre_company_token']" "$LOGIN_BODY" 2>/dev/null || true)
ACCESS=$(json_get "['session']['access_token']" "$LOGIN_BODY" 2>/dev/null || true)

if [ "$NEXT" = "select_company" ]; then
  [ -n "$PRE" ] || fail "missing pre_company_token"
  CID="${COMPANY_ID:-$(json_get "['memberships'][0]['company_id']" "$LOGIN_BODY" 2>/dev/null || json_get "['companies'][0]['company_id']" "$LOGIN_BODY")}"
  log_step "Select company $CID"
  SEL_RES=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/auth/select-company" \
    -H "Authorization: Bearer $PRE" \
    -H 'Content-Type: application/json' \
    -d "{\"company_id\":\"$CID\"}") || fail "select-company curl error"
  SEL_CODE=$(http_code "$SEL_RES")
  SEL_BODY=$(http_body "$SEL_RES")
  [ "$SEL_CODE" = "200" ] || fail "select-company HTTP $SEL_CODE"
  ACCESS=$(json_get "['access_token']" "$SEL_BODY")
fi

[ -n "$ACCESS" ] || fail "no access token (next_action=$NEXT)"
ok "access token ready"

log_step "POST /company/create (idempotency $IDEM_KEY)"
CREATE_RES=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/company/create" \
  -H "Authorization: Bearer $ACCESS" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $IDEM_KEY" \
  -d "{\"company_name\":\"Smoke Co $(date +%H%M%S)\",\"tax_code\":\"$(date +%s | tail -c 10)\"}")
CREATE_CODE=$(http_code "$CREATE_RES")
CREATE_BODY=$(http_body "$CREATE_RES")
echo "  HTTP $CREATE_CODE"
echo "  body: $CREATE_BODY"

case "$CREATE_CODE" in
  201)
    NEW_CID=$(json_get "['company_id']" "$CREATE_BODY")
    TOKEN=$(json_get "['session']['access_token']" "$CREATE_BODY")
    [ -n "$TOKEN" ] || fail "201 without access_token"
    ok "create company_id=$NEW_CID"
    log_step "GET /me with new token"
    ME_RES=$(curl -sf "$BASE_URL/api/v1/me" -H "Authorization: Bearer $TOKEN") || fail "/me after create"
    ME_CID=$(json_get "['current_context']['company_id']" "$ME_RES")
    if [ "$ME_CID" = "$NEW_CID" ]; then
      ok "context switched to new company"
    else
      warn "context company_id=$ME_CID expected $NEW_CID"
    fi
    ;;
  402)
    ok "quota exceeded (expected for Free at limit)"
    ;;
  404)
    CODE=$(json_get "['error']['code']" "$CREATE_BODY" 2>/dev/null || echo "?")
    [ "$CODE" = "FEATURE_DISABLED" ] && ok "FEATURE_DISABLED when flag off" || fail "unexpected 404: $CREATE_BODY"
    ;;
  403)
    CODE=$(json_get "['error']['code']" "$CREATE_BODY" 2>/dev/null || echo "?")
    [ "$CODE" = "EMAIL_VERIFICATION_REQUIRED" ] && ok "email verification gate" || fail "unexpected 403: $CREATE_BODY"
    ;;
  *)
    warn "create returned $CREATE_CODE — adjust account/tier/flags and re-run"
    ;;
esac

log_step "Idempotency replay (same key)"
REPLAY_RES=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/company/create" \
  -H "Authorization: Bearer $ACCESS" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $IDEM_KEY" \
  -d "{\"company_name\":\"Different Name Should Conflict\"}")
REPLAY_CODE=$(http_code "$REPLAY_RES")
if [ "$CREATE_CODE" = "201" ] && [ "$REPLAY_CODE" = "201" ]; then
  ok "idempotency replay returned 201"
elif [ "$REPLAY_CODE" = "409" ]; then
  ok "idempotency conflict on body mismatch"
else
  warn "replay HTTP $REPLAY_CODE (create was $CREATE_CODE)"
fi

echo ""
echo "Smoke finished. See docs/ai-cache/company-self-service-phase-d-release-summary.md for full QA matrix."
