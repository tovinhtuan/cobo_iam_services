#!/bin/sh
# Fix permissions for nginx bind-mount on DEV (run ON server as root).
# WEB-ONLY: must never recreate/restart API.
# Usage: sh scripts/fix-dev-web-perms.sh [/root/cobo_project]

set -eu
ROOT="${1:-/root/cobo_project}"

echo "==> Fixing web/nginx permissions under ${ROOT} (web-only)"

mkdir -p "${ROOT}/web/dist"

# Readable by nginx container (worker runs as nginx, mount is ro)
chown -R root:root "${ROOT}/web" 2>/dev/null || true
find "${ROOT}/web/dist" -type d -exec chmod 755 {} \; 2>/dev/null || true
find "${ROOT}/web/dist" -type f -exec chmod 644 {} \; 2>/dev/null || true
chmod 644 "${ROOT}/web/nginx.conf" 2>/dev/null || true

echo "==> Ensure web is up (no-deps; never touch api)"
cd "${ROOT}"
# --no-deps: do not start/recreate depends_on api
# no --force-recreate: reuse existing web container when possible after stop+start
docker compose -f docker-compose.artifacts.yml up -d --no-deps web

echo "==> Smoke checks (FE + API health probe; does not restart API)"
sleep 2
curl -s -o /dev/null -w "healthz (8080): %{http_code}\n" http://127.0.0.1:8080/healthz || true
curl -s -o /dev/null -w "login-key (8080): %{http_code}\n" http://127.0.0.1:8080/api/v1/auth/login-password-key || true
curl -s -o /dev/null -w "fe index (3000): %{http_code}\n" http://127.0.0.1:3000/ || true

docker compose -f docker-compose.artifacts.yml ps web api
