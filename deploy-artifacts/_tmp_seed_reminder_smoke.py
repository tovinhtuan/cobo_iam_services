#!/usr/bin/env python3
"""DEV-only: seed + wait for worker due-dispatch to create in-app via Bridge (post-deploy binary)."""
from __future__ import annotations

import json
import time
import urllib.request
import urllib.error
import uuid
from datetime import datetime, timedelta, timezone

BASE = "http://88.216.208.0:8080"
TOKEN = "1815e3063242d0a6d369fbb13c668e68409e95c7594418ec58e9ce2cf9688e03"
# Real c_001 record used in prior smoke needles
RECORD_ID = "52698f3f-53c0-53e7-9e40-d99f90774f41"
# Prefer email column used by UserIDsByEmails (users.email for admin.dn)
RECIPIENT = "tvttthptlvh@gmail.com"
OCC_ID = f"smoke-dash-act-{uuid.uuid4().hex[:12]}"
IDEM = f"idem-{OCC_ID}"


def req(method: str, path: str, body: dict | None = None):
    data = None if body is None else json.dumps(body).encode()
    r = urllib.request.Request(
        BASE + path,
        data=data,
        headers={
            "Content-Type": "application/json",
            "X-Internal-Token": TOKEN,
        },
        method=method,
    )
    try:
        with urllib.request.urlopen(r, timeout=60) as resp:
            raw = resp.read()
            return resp.status, json.loads(raw.decode() or "null") if raw else None
    except urllib.error.HTTPError as e:
        raw = e.read()
        try:
            return e.code, json.loads(raw.decode() or "null")
        except Exception:
            return e.code, raw.decode(errors="replace")


def main() -> None:
    # scheduled slightly in the past so worker due-tick will claim it
    scheduled = (datetime.now(timezone.utc) - timedelta(seconds=30)).strftime("%Y-%m-%dT%H:%M:%SZ")
    seed_body = {
        "disclosure_id": RECORD_ID,
        "scope_type": "WORKFLOW_STEP",
        "scope_id": "step-smoke-xac-dinh-nghia-vu",
        "scheduled_at": scheduled,
        "status": "PENDING",
        "idempotency_key": IDEM,
    }
    # SeedOccurrence may ignore occurrence_id; capture response
    st, seed = req("POST", "/internal/dev/reminders/seed-occurrence", seed_body)
    print("SEED", st, json.dumps(seed, ensure_ascii=False)[:500])
    if st not in (200, 201):
        raise SystemExit(1)
    occ = seed.get("occurrence_id") if isinstance(seed, dict) else None
    if not occ:
        # some responses wrap
        occ = (seed or {}).get("occurrence", {}).get("occurrence_id") if isinstance(seed, dict) else None
    print("OCCURRENCE_ID", occ)
    print("IDEM", IDEM)
    print("Waiting up to 90s for worker DispatchDueOccurrences + in-app bridge...")
    time.sleep(5)


if __name__ == "__main__":
    main()
