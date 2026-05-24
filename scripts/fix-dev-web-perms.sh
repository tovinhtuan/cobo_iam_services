#!/bin/sh
# Fix permissions for nginx bind-mount on dev server (run ON server as root).
# Usage: sh scripts/fix-dev-web-perms.sh [/root/cobo_project]

set -eu
ROOT="${1:-/root/cobo_project}"

echo "==> Fixing web/nginx permissions under ${ROOT}"

mkdir -p "${ROOT}/web/dist" "${ROOT}/configs"

# Readable by nginx container (worker runs as nginx, mount is ro)
chown -R root:root "${ROOT}/web" 2>/dev/null || true
find "${ROOT}/web/dist" -type d -exec chmod 755 {} \; 2>/dev/null || true
find "${ROOT}/web/dist" -type f -exec chmod 644 {} \; 2>/dev/null || true
chmod 644 "${ROOT}/web/nginx.conf" 2>/dev/null || true

if [ -f "${ROOT}/configs/login_password_rsa_dev.pem" ]; then
  chmod 600 "${ROOT}/configs/login_password_rsa_dev.pem"
fi

if [ -f "${ROOT}/bin/api" ]; then
  chmod 755 "${ROOT}/bin/api" "${ROOT}/bin/worker" 2>/dev/null || true
fi

echo "==> Restart web + api"
cd "${ROOT}"
docker compose -f docker-compose.artifacts.yml up -d --force-recreate api web

echo "==> Smoke checks"
sleep 3
curl -s -o /dev/null -w "healthz (8080): %{http_code}\n" http://127.0.0.1:8080/healthz || true
curl -s -o /dev/null -w "login-key (8080): %{http_code}\n" http://127.0.0.1:8080/api/v1/auth/login-password-key || true
curl -s -o /dev/null -w "login-key (3000): %{http_code}\n" http://127.0.0.1:3000/api/v1/auth/login-password-key || true

docker compose -f docker-compose.artifacts.yml ps
