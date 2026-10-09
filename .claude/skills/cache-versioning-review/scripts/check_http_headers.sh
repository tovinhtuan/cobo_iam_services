#!/usr/bin/env bash
# Inspect cache + security headers of a deployed Cobo web build.
# Usage: check_http_headers.sh http://host:3000
set -euo pipefail
BASE="${1:?usage: $0 <base-url>}"
BASE="${BASE%/}"
hdr() { curl -sS -o /dev/null -D - "$1" | tr -d '\r'; }
show() { echo "== $1"; hdr "$1" | grep -iE '^(HTTP/|cache-control|etag|last-modified|expires|age|content-type|content-security-policy|x-content-type-options|x-frame-options|referrer-policy|strict-transport-security)' || true; echo; }

show "$BASE/"
show "$BASE/index.html"
show "$BASE/some/deep/link"

ASSET=$(curl -sS "$BASE/" | grep -oE '/assets/[^"]+\.(js|css)' | head -n1 || true)
if [[ -n "$ASSET" ]]; then
  show "$BASE$ASSET"
  echo "== stale-asset probe (should be 404, not index.html):"
  curl -sS -o /dev/null -w '%{http_code} %{content_type}\n' "$BASE/assets/does-not-exist-123.js"
fi

echo "== verdict"
IDX=$(hdr "$BASE/index.html" | grep -i '^cache-control' || true)
[[ "$IDX" =~ no-store|no-cache ]] && echo "OK  index.html not cached" || echo "WARN index.html cacheable: $IDX"
if [[ -n "${ASSET:-}" ]]; then
  A=$(hdr "$BASE$ASSET" | grep -i '^cache-control' || true)
  [[ "$A" =~ immutable ]] && echo "OK  hashed asset immutable" || echo "WARN hashed asset: $A"
fi
for h in content-security-policy x-content-type-options x-frame-options referrer-policy; do
  hdr "$BASE/" | grep -qi "^$h" && echo "OK  $h" || echo "MISS $h"
done
