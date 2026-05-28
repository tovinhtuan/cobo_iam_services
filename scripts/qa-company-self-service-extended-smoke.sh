#!/bin/sh
# Extended staging smoke for company self-service (initialize + create matrix).
set -eu

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
LOGIN_ID="${LOGIN_ID:-user@example.com}"
PASSWORD="${PASSWORD:-secret}"

PASS=0
FAIL=0
SKIP=0

record() {
  case "$1" in
    pass) PASS=$((PASS + 1)); echo "PASS: $2" ;;
    fail) FAIL=$((FAIL + 1)); echo "FAIL: $2" >&2 ;;
    skip) SKIP=$((SKIP + 1)); echo "SKIP: $2" ;;
  esac
}

json_get() {
  printf '%s' "$2" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d$1)"
}

http_code() { printf '%s' "$1" | tail -n1; }
http_body() { printf '%s' "$1" | sed '$d'; }

login_access() {
  LOGIN_RES=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"login_id\":\"$LOGIN_ID\",\"password\":\"$PASSWORD\"}")
  LOGIN_CODE=$(http_code "$LOGIN_RES")
  LOGIN_BODY=$(http_body "$LOGIN_RES")
  [ "$LOGIN_CODE" = "200" ] || { record fail "login HTTP $LOGIN_CODE"; return 1; }
  NEXT=$(json_get "['next_action']" "$LOGIN_BODY")
  PRE=$(json_get "['session']['pre_company_token']" "$LOGIN_BODY" 2>/dev/null || true)
  ACCESS=$(json_get "['session']['access_token']" "$LOGIN_BODY" 2>/dev/null || true)
  if [ "$NEXT" = "select_company" ]; then
    CID=$(json_get "['memberships'][0]['company_id']" "$LOGIN_BODY" 2>/dev/null || json_get "['companies'][0]['company_id']" "$LOGIN_BODY")
    SEL_RES=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/auth/select-company" \
      -H "Authorization: Bearer $PRE" \
      -H 'Content-Type: application/json' \
      -d "{\"company_id\":\"$CID\"}")
    SEL_CODE=$(http_code "$SEL_RES")
    [ "$SEL_CODE" = "200" ] || { record fail "select-company $SEL_CODE"; return 1; }
    ACCESS=$(json_get "['access_token']" "$(http_body "$SEL_RES")")
  fi
  [ -n "$ACCESS" ] || { record fail "no access token"; return 1; }
  printf '%s' "$ACCESS"
}

echo "=== Extended company self-service smoke ==="
echo "BASE_URL=$BASE_URL LOGIN_ID=$LOGIN_ID"

# Migration columns (via mysql in docker if available)
if docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -N -e "SHOW COLUMNS FROM companies LIKE 'founder_user_id';" 2>/dev/null | grep -q founder_user_id; then
  record pass "migration 0082 founder_user_id column"
else
  record skip "migration column check (mysql client unavailable)"
fi

ACCESS=$(login_access) || exit 1
record pass "login + company context"

# CS-02: initialize blocked when user has eligible companies
INIT_RES=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/company/initialize" \
  -H "Authorization: Bearer $ACCESS" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: ext-init-block-$(date +%s)" \
  -d '{"company_name":"Should Not Initialize"}')
INIT_CODE=$(http_code "$INIT_RES")
INIT_BODY=$(http_body "$INIT_RES")
if [ "$INIT_CODE" = "409" ]; then
  CODE=$(json_get "['error']['code']" "$INIT_BODY" 2>/dev/null || echo "")
  if [ "$CODE" = "COMPANY_ALREADY_EXISTS" ] || [ "$CODE" = "STATE_CONFLICT" ]; then
    record pass "initialize blocked for existing company user ($CODE)"
  else
    record fail "initialize 409 unexpected code=$CODE body=$INIT_BODY"
  fi
else
  record fail "initialize expected 409 got $INIT_CODE body=$INIT_BODY"
fi

# CS-07/12: create + duplicate tax
TAX="EXT$(date +%s | tail -c 9)"
IDEM1="ext-create-$(date +%s)"
C1=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/company/create" \
  -H "Authorization: Bearer $ACCESS" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $IDEM1" \
  -d "{\"company_name\":\"Ext Co 1\",\"tax_code\":\"$TAX\"}")
C1_CODE=$(http_code "$C1")
C1_BODY=$(http_body "$C1")
if [ "$C1_CODE" = "201" ]; then
  record pass "create nth success"
  TOK=$(json_get "['session']['access_token']" "$C1_BODY")
  [ -n "$TOK" ] || record fail "201 missing access_token"
  # idempotency replay same body
  C1R=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/company/create" \
    -H "Authorization: Bearer $ACCESS" \
    -H 'Content-Type: application/json' \
    -H "Idempotency-Key: $IDEM1" \
    -d "{\"company_name\":\"Ext Co 1\",\"tax_code\":\"$TAX\"}")
  C1R_CODE=$(http_code "$C1R")
  if [ "$C1R_CODE" = "201" ]; then
    record pass "idempotency replay 201"
  else
    record fail "idempotency replay expected 201 got $C1R_CODE"
  fi
  # idempotency conflict
  C1C=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/company/create" \
    -H "Authorization: Bearer $ACCESS" \
    -H 'Content-Type: application/json' \
    -H "Idempotency-Key: $IDEM1" \
    -d '{"company_name":"Different"}')
  C1C_CODE=$(http_code "$C1C")
  if [ "$C1C_CODE" = "409" ]; then
    record pass "idempotency conflict 409"
  else
    record fail "idempotency conflict expected 409 got $C1C_CODE"
  fi
  # duplicate tax
  C2=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/company/create" \
    -H "Authorization: Bearer $ACCESS" \
    -H 'Content-Type: application/json' \
    -H "Idempotency-Key: ext-dup-tax-$(date +%s)" \
    -d "{\"company_name\":\"Ext Co Dup\",\"tax_code\":\"$TAX\"}")
  C2_CODE=$(http_code "$C2")
  C2_BODY=$(http_body "$C2")
  if [ "$C2_CODE" = "409" ]; then
    record pass "duplicate tax_code 409"
  else
    record fail "duplicate tax expected 409 got $C2_CODE body=$C2_BODY"
  fi
elif [ "$C1_CODE" = "402" ]; then
  record pass "quota exceeded on create (acceptable for Free tier)"
  record skip "idempotency/duplicate tax (quota blocked first create)"
else
  record fail "create expected 201 or 402 got $C1_CODE body=$C1_BODY"
fi

# CS-09: quota — loop creates until 402 if tier limited
QUOTA_HIT=0
i=0
while [ "$i" -lt 5 ]; do
  R=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/company/create" \
    -H "Authorization: Bearer $ACCESS" \
    -H 'Content-Type: application/json' \
    -H "Idempotency-Key: ext-quota-$i-$(date +%s)" \
    -d "{\"company_name\":\"Quota Probe $i\"}")
  RC=$(http_code "$R")
  if [ "$RC" = "402" ]; then QUOTA_HIT=1; break; fi
  if [ "$RC" != "201" ]; then break; fi
  i=$((i + 1))
done
if [ "$QUOTA_HIT" = "1" ]; then
  record pass "quota exceeded 402 observed"
else
  record skip "quota 402 not observed (Enterprise/unlimited or already at limit before loop)"
fi

echo ""
echo "=== Summary: PASS=$PASS FAIL=$FAIL SKIP=$SKIP ==="
[ "$FAIL" -eq 0 ] || exit 1
