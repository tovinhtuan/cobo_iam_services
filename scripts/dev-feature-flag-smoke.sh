#!/bin/sh
# Temporarily disable COMPANY_SELF_CREATE_ENABLED and verify FEATURE_DISABLED.
set -eu
BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
LOGIN_ID="${LOGIN_ID}"
PASSWORD="${PASSWORD}"

json_get() {
  printf '%s' "$2" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d$1)"
}
http_code() { printf '%s' "$1" | tail -n1; }
http_body() { printf '%s' "$1" | sed '$d'; }

echo "Stopping API to flip flag OFF..."
docker stop cobo-iam-api >/dev/null

# Run one-off container with flag false (same image/network)
docker run --rm -d --name cobo-iam-api-flagtest \
  --network container:cobo-iam-redis 2>/dev/null || true

# Simpler: patch env in compose and recreate
sed -i 's/COMPANY_SELF_CREATE_ENABLED=true/COMPANY_SELF_CREATE_ENABLED=false/' /root/cobo_project/.env
cd /root/cobo_project
docker compose -f docker-compose.artifacts.yml up -d api
sleep 3
curl -sf "$BASE_URL/healthz" >/dev/null || { echo "health failed"; exit 1; }

LOGIN_RES=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"login_id\":\"$LOGIN_ID\",\"password\":\"$PASSWORD\"}")
ACCESS=$(json_get "['session']['access_token']" "$(http_body "$LOGIN_RES")" 2>/dev/null || true)
NEXT=$(json_get "['next_action']" "$(http_body "$LOGIN_RES")")
if [ "$NEXT" = "select_company" ]; then
  PRE=$(json_get "['session']['pre_company_token']" "$(http_body "$LOGIN_RES")")
  CID=$(json_get "['memberships'][0]['company_id']" "$(http_body "$LOGIN_RES")")
  SEL=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/auth/select-company" \
    -H "Authorization: Bearer $PRE" -H 'Content-Type: application/json' \
    -d "{\"company_id\":\"$CID\"}")
  ACCESS=$(json_get "['access_token']" "$(http_body "$SEL")")
fi

RES=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/company/create" \
  -H "Authorization: Bearer $ACCESS" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: flag-off-$(date +%s)" \
  -d '{"company_name":"Flag Off Test"}')
CODE=$(http_code "$RES")
BODY=$(http_body "$RES")
echo "create HTTP $CODE body=$BODY"

sed -i 's/COMPANY_SELF_CREATE_ENABLED=false/COMPANY_SELF_CREATE_ENABLED=true/' /root/cobo_project/.env
docker compose -f docker-compose.artifacts.yml up -d api
sleep 2

if [ "$CODE" = "404" ]; then
  ERR=$(json_get "['error']['code']" "$BODY" 2>/dev/null || echo "")
  if [ "$ERR" = "FEATURE_DISABLED" ]; then
    echo "PASS: FEATURE_DISABLED when flag off"
    exit 0
  fi
fi
echo "FAIL: expected 404 FEATURE_DISABLED"
exit 1
