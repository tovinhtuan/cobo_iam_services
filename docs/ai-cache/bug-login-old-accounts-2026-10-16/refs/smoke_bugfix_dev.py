#!/usr/bin/env python3
"""DEV smoke for bug-login-old-accounts-2026-10-16 (BUG-1, BUG-2). Prints statuses and counts only,
never addresses or secrets.

Run from cobo_iam_services:
  python3 docs/ai-cache/bug-login-old-accounts-2026-10-16/refs/smoke_bugfix_dev.py
Side effects on DEV: 3 failed and 1 successful sign-in rows (login_attempts, audit_logs) and one
password-reset token + email for the USER QA persona; nothing else.
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
results = []


def post(path, body, user_agent="cobo-smoke-bugfix/1"):
    req = urllib.request.Request(BASE + path, data=json.dumps(body).encode(), method="POST",
                                 headers={"Content-Type": "application/json", "User-Agent": user_agent})
    try:
        with urllib.request.urlopen(req, timeout=20) as r:
            return r.status
    except urllib.error.HTTPError as e:
        return e.code


def one(sql):
    rows = devqa_env.run_sql(sql)
    return rows[0] if rows else ""


def check(name, ok, detail):
    results.append({"step": name, "ok": bool(ok), "detail": detail})
    print(("PASS " if ok else "FAIL ") + name + " | " + detail)


def attempt(login_id):
    """success, failure_code, user_id set?, user_agent has U+FFFD? of the newest row for login_id."""
    return one(
        "SELECT CONCAT_WS('|', success, IFNULL(failure_code,'-'), user_id IS NOT NULL, "
        "LOCATE(CONVERT(0xEFBFBD USING utf8mb4), IFNULL(user_agent,'')) > 0) "
        f"FROM login_attempts WHERE login_id = '{login_id}' ORDER BY attempt_id DESC LIMIT 1;"
    )


def audit_failures(login_id):
    return int(one(
        "SELECT COUNT(*) FROM audit_logs WHERE action = 'login_failure' AND "
        f"JSON_UNQUOTE(JSON_EXTRACT(metadata_json, '$.login_id')) = '{login_id}';"
    ) or 0)


tag = uuid.uuid4().hex[:8]

# BUG-1a: unknown account, sent as `email` like the web form.
unknown = f"smoke-bugfix-{tag}@cobo.test"
status = post("/api/v1/auth/login", {"email": unknown.upper(), "password": "not-the-password"})
time.sleep(1)
check("failed-login-unknown-recorded", status == 401 and attempt(unknown) == "0|INVALID_CREDENTIALS|0|0",
      f"status={status} attempt={attempt(unknown)}")
check("failed-login-unknown-audited", audit_failures(unknown) == 1, f"audit rows={audit_failures(unknown)}")

# BUG-1b: existing account, wrong password.
user_email, _ = devqa_env.persona("USER")
uid = one(f"SELECT user_id FROM users WHERE LOWER(login_id) = '{user_email.lower()}' LIMIT 1;")
status = post("/api/v1/auth/login", {"email": " " + user_email + " ", "password": "not-the-password"})
time.sleep(1)
check("failed-login-persona-recorded", status == 401 and attempt(user_email.lower()).startswith("0|INVALID_CREDENTIALS|"),
      f"status={status} attempt={attempt(user_email.lower())}")

# Review BES-04: a raw 0xFF byte in the User-Agent must not drop the row.
odd = f"smoke-bugfix-ua-{tag}@cobo.test"
status = post("/api/v1/auth/login", {"email": odd, "password": "x"}, user_agent="cobo-smoke-\xff-ua")
time.sleep(1)
check("failed-login-odd-user-agent-recorded", status == 401 and attempt(odd) == "0|INVALID_CREDENTIALS|0|1",
      f"status={status} attempt={attempt(odd)} audit rows={audit_failures(odd)} (audit repo not sanitised yet)")

# Sign-in still works and is recorded.
ent_email, ent_password = devqa_env.persona("ENT")
before = int(one(f"SELECT COUNT(*) FROM login_attempts WHERE login_id = '{ent_email.lower()}' AND success = 1;") or 0)
status = post("/api/v1/auth/login", {"email": ent_email, "password": ent_password})
time.sleep(1)
after = int(one(f"SELECT COUNT(*) FROM login_attempts WHERE login_id = '{ent_email.lower()}' AND success = 1;") or 0)
check("login-ok-recorded", status == 200 and after == before + 1, f"status={status} success rows +{after - before}")

# BUG-2: one mailbox, one account -> exactly one token and one mail to that address.
tokens_before = int(one(f"SELECT COUNT(*) FROM password_reset_tokens WHERE user_id = '{uid}';") or 0)
all_before = int(one("SELECT COUNT(*) FROM password_reset_tokens;") or 0)
status = post("/api/v1/auth/forgot-password", {"email": user_email.upper()})
event = ""
for _ in range(20):
    time.sleep(3)
    event = one(
        "SELECT CONCAT_WS('|', status, JSON_UNQUOTE(JSON_EXTRACT(payload_json, '$.to')) = "
        f"(SELECT IFNULL(NULLIF(email,''), login_id) FROM users WHERE user_id = '{uid}')) FROM outbox_events "
        f"WHERE event_type = 'auth.password_reset_requested' AND aggregate_id = '{uid}' "
        "AND created_at > UTC_TIMESTAMP() - INTERVAL 2 MINUTE ORDER BY created_at DESC LIMIT 1;"
    )
    if event.startswith("processed"):
        break
tokens_after = int(one(f"SELECT COUNT(*) FROM password_reset_tokens WHERE user_id = '{uid}';") or 0)
check("forgot-one-account", status == 200 and tokens_after == tokens_before + 1 and event == "processed|1",
      f"status={status} tokens +{tokens_after - tokens_before} event={event or 'none'}")

# Unknown address: generic 200, nothing issued.
status = post("/api/v1/auth/forgot-password", {"email": f"smoke-bugfix-nobody-{tag}@cobo.test"})
time.sleep(1)
all_after = int(one("SELECT COUNT(*) FROM password_reset_tokens;") or 0)
check("forgot-unknown-silent", status == 200 and all_after == all_before + 1, f"status={status} tokens total +{all_after - all_before}")

# API log: no lookup errors, no panic.
r = subprocess.run(devqa_env.ssh_cmd() + [
    "docker logs --since 10m cobo-iam-api 2>&1 | grep -cE 'password_reset_lookup_failed|panic' || true"],
    capture_output=True, text=True, timeout=60)
bad = int((r.stdout.strip() or "0").splitlines()[-1])
check("api-log-clean", bad == 0, f"lookup_failed/panic lines={bad}")

out = os.path.join(os.path.dirname(__file__), "smoke-bugfix-dev-result.json")
with open(out, "w", encoding="utf-8") as f:
    json.dump({"base": "DEV", "results": results}, f, ensure_ascii=False, indent=2)
print(f"{sum(r['ok'] for r in results)}/{len(results)} passed")
sys.exit(0 if all(r["ok"] for r in results) else 1)
