#!/bin/sh
# Usage: dev-toggle-create-flag.sh on|off
set -eu
COMPOSE=/root/cobo_project/docker-compose.artifacts.yml
case "${1:-}" in
  on)
    sed -i 's/COMPANY_SELF_CREATE_ENABLED: "false"/COMPANY_SELF_CREATE_ENABLED: "true"/' "$COMPOSE"
    sed -i 's/COMPANY_SELF_CREATE_ENABLED=false/COMPANY_SELF_CREATE_ENABLED=true/' /root/cobo_project/.env
    ;;
  off)
    sed -i 's/COMPANY_SELF_CREATE_ENABLED: "true"/COMPANY_SELF_CREATE_ENABLED: "false"/' "$COMPOSE"
    sed -i 's/COMPANY_SELF_CREATE_ENABLED=true/COMPANY_SELF_CREATE_ENABLED=false/' /root/cobo_project/.env
    ;;
  *)
    echo "usage: $0 on|off" >&2
    exit 2
    ;;
esac
cd /root/cobo_project
docker compose -f docker-compose.artifacts.yml up -d --force-recreate api
sleep 4
docker exec cobo-iam-api printenv COMPANY_SELF_CREATE_ENABLED
