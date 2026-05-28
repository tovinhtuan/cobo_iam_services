#!/usr/bin/env python3
import json
import os
import sys
import urllib.error
import urllib.request

base = os.environ.get("BASE_URL", "http://127.0.0.1:8080")
login_id = os.environ["LOGIN_ID"]
password = os.environ["PASSWORD"]


def post(path, body, headers=None):
    headers = {**(headers or {}), "Content-Type": "application/json"}
    req = urllib.request.Request(
        f"{base}{path}",
        data=json.dumps(body).encode(),
        headers=headers,
        method="POST",
    )
    try:
        with urllib.request.urlopen(req) as resp:
            return json.load(resp), resp.status
    except urllib.error.HTTPError as e:
        return json.loads(e.read().decode() or "{}"), e.code


login, _ = post("/api/v1/auth/login", {"login_id": login_id, "password": password})
pre = login["session"]["pre_company_token"]
cid = login["memberships"][0]["company_id"]
sel, _ = post(
    "/api/v1/auth/select-company",
    {"company_id": cid},
    {"Authorization": f"Bearer {pre}"},
)
tok = sel["access_token"]
req = urllib.request.Request(
    f"{base}/api/v1/company/create",
    data=json.dumps({"company_name": "Flag Off Test"}).encode(),
    headers={
        "Authorization": f"Bearer {tok}",
        "Content-Type": "application/json",
        "Idempotency-Key": "flag-off-check",
    },
    method="POST",
)
try:
    with urllib.request.urlopen(req) as resp:
        print("UNEXPECTED 2xx", resp.status, resp.read().decode())
        sys.exit(1)
except urllib.error.HTTPError as e:
    body = json.loads(e.read().decode() or "{}")
    print("status", e.code, "code", body.get("error", {}).get("code"))
    if e.code == 404 and body.get("error", {}).get("code") == "FEATURE_DISABLED":
        print("PASS")
        sys.exit(0)
    print("FAIL", body)
    sys.exit(1)
