#!/usr/bin/env python3
import json
import os
import urllib.request

base = os.environ.get("BASE_URL", "http://127.0.0.1:8080")
login_id = os.environ["LOGIN_ID"]
password = os.environ["PASSWORD"]

req = urllib.request.Request(
    f"{base}/api/v1/auth/login",
    data=json.dumps({"login_id": login_id, "password": password}).encode(),
    headers={"Content-Type": "application/json"},
    method="POST",
)
with urllib.request.urlopen(req) as resp:
    data = json.load(resp)

print(json.dumps(data, indent=2)[:2000])
