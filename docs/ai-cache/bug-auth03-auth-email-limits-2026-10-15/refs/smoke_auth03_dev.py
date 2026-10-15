#!/usr/bin/env python3
"""DEV smoke for AUTH-03 (auth email limits). Prints statuses and counts only, never emails or secrets.

Run from cobo_iam_services:  python3 docs/ai-cache/bug-auth03-auth-email-limits-2026-10-15/refs/smoke_auth03_dev.py
Side effects on DEV: up to 3 password-reset tokens (and emails) for the USER QA persona; nothing else.
"""
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request
import uuid

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "..", "scripts", "devqa"))
import devqa_env  # noqa: E402

BASE = devqa_env.base_url().rstrip("/")
REFILL_WAIT = 75  # nginx auth_email_per_ip: 3r/m, burst 3


def post(path, body, token=None):
    req = urllib.request.Request(BASE + path, data=json.dumps(body).encode(), method="POST",
                                 headers={"Content-Type": "application/json"})
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=20) as r:
            return r.status, json.loads(r.read() or b"{}")
    except urllib.error.HTTPError as e:
        return e.code, None


def one(sql):
    rows = devqa_env.run_sql(sql)
    return int(rows[0]) if rows else 0


def user_id_for(email):
    rows = devqa_env.run_sql(f"SELECT user_id FROM users WHERE login_id = '{email}' LIMIT 1;")
    return rows[0] if rows else ""


results = []


def check(name, ok, detail):
    results.append({"step": name, "ok": bool(ok), "detail": detail})
    print(("PASS " if ok else "FAIL ") + name + " | " + detail)


user_email, _ = devqa_env.persona("USER")
uid = user_id_for(user_email)
check("persona-user-exists", bool(uid), "USER persona found" if uid else "USER persona missing")

# 1. Signed-out resend: generic 200 and no OTP row.
otp_before = one(f"SELECT COUNT(*) FROM email_verification_otps WHERE user_id = '{uid}';")
status, body = post("/api/v1/auth/resend-verification-email", {"email": user_email})
otp_after = one(f"SELECT COUNT(*) FROM email_verification_otps WHERE user_id = '{uid}';")
check("resend-signed-out-no-send", status == 200 and (body or {}).get("success") is True and otp_after == otp_before,
      f"status={status} otp_rows_delta={otp_after - otp_before}")

time.sleep(REFILL_WAIT)

# 2. Forgot-password per-user limit (3/hour, 10/day): 4 requests, at most the remaining quota is issued.
h0 = one(f"SELECT COUNT(*) FROM password_reset_tokens WHERE user_id = '{uid}' AND created_at >= UTC_TIMESTAMP() - INTERVAL 1 HOUR;")
d0 = one(f"SELECT COUNT(*) FROM password_reset_tokens WHERE user_id = '{uid}' AND created_at >= UTC_TIMESTAMP() - INTERVAL 1 DAY;")
expected = max(0, min(3 - h0, 10 - d0, 4))
statuses = [post("/api/v1/auth/forgot-password", {"email": user_email})[0] for _ in range(4)]
h1 = one(f"SELECT COUNT(*) FROM password_reset_tokens WHERE user_id = '{uid}' AND created_at >= UTC_TIMESTAMP() - INTERVAL 1 HOUR;")
check("forgot-per-user-limit", statuses == [200] * 4 and h1 - h0 == expected and h1 <= 3,
      f"statuses={statuses} tokens_last_hour {h0}->{h1} expected_issued={expected}")

time.sleep(REFILL_WAIT)

# 3. nginx per-IP limit with an unknown email (no token, no email): a burst of 5 gets 429 after 3-4
#    requests (a known IP gets the burst of 3, a new IP one more).
unknown = f"smoke-{uuid.uuid4().hex[:10]}@example.invalid"
statuses = [post("/api/v1/auth/forgot-password", {"email": unknown})[0] for _ in range(5)]
passed = statuses.index(429) if 429 in statuses else len(statuses)
check("nginx-per-ip-limit", passed in (3, 4) and all(s == 429 for s in statuses[passed:]), f"statuses={statuses}")

# 4. Login is not affected by the auth email zone.
ent_email, ent_password = devqa_env.persona("ENT")
statuses = [post("/api/v1/auth/login", {"email": ent_email, "password": ent_password})[0] for _ in range(2)]
check("login-unaffected", statuses == [200, 200], f"statuses={statuses}")

# 5. API log shows the limited forgot request (event name only).
r = subprocess.run(devqa_env.ssh_cmd() + ["docker logs --since 10m cobo-iam-api 2>&1 | grep -c password_reset_rate_limited"],
                   capture_output=True, text=True, timeout=60)
limited_logs = int((r.stdout.strip() or "0").splitlines()[-1])
check("rate-limited-log", limited_logs >= (4 - expected), f"password_reset_rate_limited lines={limited_logs}")

out = os.path.join(os.path.dirname(__file__), "smoke-auth03-dev-result.json")
with open(out, "w", encoding="utf-8") as f:
    json.dump({"base": "DEV", "results": results}, f, ensure_ascii=False, indent=2)
print(f"{sum(r['ok'] for r in results)}/{len(results)} passed")
sys.exit(0 if all(r["ok"] for r in results) else 1)
