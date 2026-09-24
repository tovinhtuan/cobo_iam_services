#!/usr/bin/env python3
"""Global CMS Record DEV smoke (API). No secrets written to evidence."""
from __future__ import annotations

import json
import re
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path

BASE = "http://88.216.208.0:8080"
OUT = Path(
    "/home/icom/go/src/myself/backend_api_cobo/cobo_iam_services/docs/ai-cache/cms-global-record-smoke-qa-2026-09-24"
)
EMAIL = "platform.tenant.admin@example.com"
PASSWORD = "secret"
COMPANY = "c_001"
CYCLE = "monthly:2026-09"  # must match template frequency; after applicable_from 2026-08
TITLE = "SMOKE-GLOBAL-CMS-2026-09-24"
PREFERRED_TEMPLATE = "bang-tinh-luong-nhan-vien-ban-sao-2"


def req(method: str, path: str, body=None, token=None):
    data = None
    headers = {"Accept": "application/json"}
    if body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    if token:
        headers["Authorization"] = f"Bearer {token}"
    request = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(request, timeout=90) as resp:
            raw = resp.read()
            return resp.status, json.loads(raw.decode() or "{}") if raw else {}
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        try:
            payload = json.loads(raw) if raw else {}
        except Exception:
            payload = {"raw": raw[:2000]}
        return e.code, payload


def redact(obj) -> str:
    s = json.dumps(obj, ensure_ascii=False, indent=2)
    s = re.sub(r'("access_token"\s*:\s*")[^"]+(")', r"\1[REDACTED]\2", s)
    s = re.sub(r'("refresh_token"\s*:\s*")[^"]+(")', r"\1[REDACTED]\2", s)
    s = re.sub(r'("ct"\s*:\s*")[^"]{16,}(")', r"\1[REDACTED]\2", s)
    return s


def pick_token(payload: dict) -> str | None:
    root = payload.get("data") or payload
    if not isinstance(root, dict):
        return None
    if root.get("access_token"):
        return root["access_token"]
    tokens = root.get("tokens") or {}
    if isinstance(tokens, dict) and tokens.get("access_token"):
        return tokens["access_token"]
    session = root.get("session") or {}
    if isinstance(session, dict):
        for key in ("access_token", "pre_company_token"):
            if session.get(key):
                return session[key]
    return None


def main() -> int:
    OUT.mkdir(parents=True, exist_ok=True)
    (OUT / "00-environment.md").write_text(
        f"""# Environment

- API: {BASE}
- FE: http://88.216.208.0:3000
- Account: {EMAIL} (password redacted)
- Company context: {COMPANY}
- Smoke title: {TITLE}
- cycle_key: {CYCLE}
""",
        encoding="utf-8",
    )

    st, login = req(
        "POST",
        "/api/v1/auth/login",
        {"login_id": EMAIL, "password": PASSWORD, "remember_me": False},
    )
    print("login", st)
    assert st == 200, login
    access = pick_token(login)
    assert access, login

    # Resolve membership for c_001 when present
    root = login.get("data") or login
    memberships = root.get("memberships") or []
    membership_id = None
    for m in memberships:
        if isinstance(m, dict) and m.get("company_id") == COMPANY:
            membership_id = m.get("membership_id") or m.get("id")
            break
    sel_body = {"company_id": COMPANY}
    if membership_id:
        sel_body["membership_id"] = membership_id
    st, sel = req("POST", "/api/v1/auth/select-company", sel_body, token=access)
    print("select-company", st, "membership", membership_id)
    if st == 200:
        access = pick_token(sel) or access

    st, probe = req("GET", "/api/v1/platform/cms/templates/__probe__/records?page=1", token=access)
    (OUT / "01-migration-status.txt").write_text(
        f"endpoint_probe_status={st}\n{redact(probe)}\n",
        encoding="utf-8",
    )

    q = (
        "SELECT t.type_id FROM disclosure_types t "
        "LEFT JOIN disclosure_type_versions v ON v.type_id=t.type_id AND v.version_no=t.active_version_no "
        "WHERE (t.company_id IS NULL OR t.company_id='') AND t.active_version_no>0 "
        "AND LOWER(COALESCE(t.status,''))<>'archived' "
        "AND LOWER(COALESCE(JSON_UNQUOTE(JSON_EXTRACT(v.deadline_config_json, '$.frequency_unit')),''))='monthly' "
        "ORDER BY CASE WHEN t.type_id='"
        + PREFERRED_TEMPLATE
        + "' THEN 0 ELSE 1 END, t.type_id LIMIT 5"
    )
    out = subprocess.check_output(
        [
            "ssh",
            "-p",
            "21239",
            "-o",
            "BatchMode=yes",
            "root@88.216.208.0",
            f"docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -Nse \"{q}\"",
        ],
        text=True,
    )
    template_ids = [ln.strip() for ln in out.splitlines() if ln.strip() and "Warning" not in ln]
    assert template_ids, "no monthly global template on DEV"
    template_id = template_ids[0]
    print("template_id", template_id, "cycle", CYCLE)

    st, hist = req(
        "GET",
        f"/api/v1/platform/cms/templates/{template_id}/records?cycle_key={CYCLE}&page=1&page_size=20",
        token=access,
    )
    print("list history", st)
    items = ((hist.get("data") or {}).get("items") if isinstance(hist, dict) else None) or []
    for it in items:
        if it.get("status") in ("Draft", "Published") and it.get("cycle_key") == CYCLE:
            st_a, _ = req("POST", f"/api/v1/platform/cms/records/{it['id']}/archive", {}, token=access)
            print("archive prior", it["id"], st_a)

    st, created = req(
        "POST",
        f"/api/v1/platform/cms/templates/{template_id}/records",
        {
            "title": TITLE,
            "summary": "smoke global cms",
            "content": "<p>SMOKE-GLOBAL-CMS-2026-09-24 content</p>",
            "cycle_key": CYCLE,
        },
        token=access,
    )
    print("create", st)
    (OUT / "02-template-record-create.json").write_text(redact({"status": st, "body": created}), encoding="utf-8")
    assert st in (200, 201), created
    rec = created.get("data") or created
    rec_id = rec["id"]

    st_dup, dup = req(
        "POST",
        f"/api/v1/platform/cms/templates/{template_id}/records",
        {"title": TITLE + "-dup", "summary": "x", "content": "y", "cycle_key": CYCLE},
        token=access,
    )
    print("duplicate", st_dup)

    st, pub = req("POST", f"/api/v1/platform/cms/records/{rec_id}/publish", {}, token=access)
    print("publish", st)
    (OUT / "03-publish.json").write_text(
        redact({"status": st, "body": pub, "duplicate_status": st_dup, "duplicate_body": dup}),
        encoding="utf-8",
    )
    assert st == 200, pub
    assert (pub.get("data") or pub).get("status") == "Published"

    st_up, up = req(
        "PUT",
        f"/api/v1/platform/cms/records/{rec_id}",
        {"title": "x", "content": "y"},
        token=access,
    )
    print("update published", st_up)

    st, prev = req(
        "POST",
        f"/api/v1/platform/cms/records/{rec_id}/materialize",
        {"mode": "full", "dry_run": True, "company_ids": []},
        token=access,
    )
    print("preview", st)
    (OUT / "04-eligibility-preview.json").write_text(redact({"status": st, "body": prev}), encoding="utf-8")
    assert st == 200, prev

    st, mat = req(
        "POST",
        f"/api/v1/platform/cms/records/{rec_id}/materialize",
        {"mode": "full", "dry_run": False, "company_ids": []},
        token=access,
    )
    print("materialize", st)
    (OUT / "05-materialize-run.json").write_text(redact({"status": st, "body": mat}), encoding="utf-8")
    assert st == 200, mat
    mat_data = mat.get("data") or mat

    st, kids = req(
        "GET",
        f"/api/v1/platform/cms/records/{rec_id}/company-records?page=1&page_size=50",
        token=access,
    )
    print("company-records", st)
    (OUT / "06-company-records.json").write_text(redact({"status": st, "body": kids}), encoding="utf-8")
    kid_items = ((kids.get("data") or {}).get("items") if isinstance(kids, dict) else []) or []
    statuses = sorted({i.get("status") for i in kid_items})

    st, mat2 = req(
        "POST",
        f"/api/v1/platform/cms/records/{rec_id}/materialize",
        {"mode": "full", "dry_run": False},
        token=access,
    )
    print("rematerialize", st)

    # Flow D: submit a NotStarted company child → PendingReview (must not put Global in queue)
    submit_status = None
    submit_body = None
    child_c001 = next((i for i in kid_items if i.get("company_id") == COMPANY), None)
    if child_c001 and child_c001.get("record_id"):
        st_sub, submit_body = req(
            "POST",
            f"/api/v1/disclosures/{child_c001['record_id']}/submit",
            {},
            token=access,
        )
        submit_status = st_sub
        print("submit child", st_sub, child_c001["record_id"])

    st, reviews = req("GET", "/api/v1/platform/cms/reviews", token=access)
    print("reviews", st)
    (OUT / "07-company-queue.json").write_text(
        redact({"status": st, "body": reviews, "submit_status": submit_status, "submit_body": submit_body}),
        encoding="utf-8",
    )
    rev_items = ((reviews.get("data") or {}).get("items") if isinstance(reviews, dict) else []) or []
    global_in_queue = [r for r in rev_items if r.get("entry_id") == rec_id or r.get("id") == rec_id]
    child_in_queue = []
    if child_c001:
        rid = child_c001.get("record_id")
        child_in_queue = [
            r
            for r in rev_items
            if r.get("entry_id") == rid or r.get("record_id") == rid or r.get("id") == rid
        ]

    # Archive after smoke to free cycle_key for re-runs (optional cleanup of smoke prefix only)
    st_arch, arch = req("POST", f"/api/v1/platform/cms/records/{rec_id}/archive", {}, token=access)
    print("archive smoke record", st_arch)

    (OUT / "08-permission-negative-cases.json").write_text(
        redact(
            {
                "update_published_status": st_up,
                "update_published_body": up,
                "duplicate_create_status": st_dup,
                "archive_after_smoke": st_arch,
                "archive_body": arch,
                "note": "Platform admin has cms.record.*; negatives are Published immutability + duplicate cycle_key.",
            }
        ),
        encoding="utf-8",
    )

    prev_data = prev.get("data") or prev
    reason_codes = sorted(
        {
            (r.get("reason_code") or r.get("error_code") or "")
            for r in (prev_data.get("results") or [])
            if isinstance(r, dict)
        }
    )
    summary = {
        "template_id": template_id,
        "cycle_key": CYCLE,
        "record_id": rec_id,
        "publish_status": (pub.get("data") or pub).get("status"),
        "preview_created": prev_data.get("created_count"),
        "preview_skipped": prev_data.get("skipped_count"),
        "preview_eligible": prev_data.get("eligible_count"),
        "preview_reason_codes": reason_codes,
        "materialize_created": mat_data.get("created_count"),
        "materialize_skipped": mat_data.get("skipped_count"),
        "materialize_exists_retry": (mat2.get("data") or mat2).get("exists_count"),
        "company_child_count": len(kid_items),
        "company_child_statuses": statuses,
        "c001_child_status_before_submit": (child_c001 or {}).get("status"),
        "submit_http": submit_status,
        "child_in_review_queue": len(child_in_queue),
        "global_in_review_queue": len(global_in_queue),
        "update_published_http": st_up,
        "duplicate_http": st_dup,
        "archive_http": st_arch,
    }
    (OUT / "09-summary.md").write_text(
        "# Smoke summary\n\n```json\n" + json.dumps(summary, indent=2, ensure_ascii=False) + "\n```\n",
        encoding="utf-8",
    )
    print("SUMMARY", json.dumps(summary, ensure_ascii=False))
    ok = (
        summary["publish_status"] == "Published"
        and summary["update_published_http"] == 409
        and summary["duplicate_http"] == 409
        and summary["global_in_review_queue"] == 0
        and "NotStarted" in statuses
        and (summary["materialize_created"] or 0) >= 1
        and (summary["preview_eligible"] or 0) >= 1
        and "ENTITLEMENT_MISSING" in reason_codes
    )
    return 0 if ok else 2


if __name__ == "__main__":
    raise SystemExit(main())
