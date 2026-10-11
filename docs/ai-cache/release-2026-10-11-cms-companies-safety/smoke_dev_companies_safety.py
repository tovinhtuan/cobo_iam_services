"""DEV API smoke for CMS companies safety (Đợt 1, 2026-10-11).

Creates a disposable company `QA Smoke <b>Status</b> <ts>` (test-owned data, left inactive at the end),
then checks: validation + duplicate tax code, revision/STALE_ETAG, status reasons and transitions,
confirm code, meta.status_change, history, audit rows in MySQL (PII masked), emails in Mailpit
(email_notifications rows, no internal note), enterprise admin 403, body limit 413. Personas
from ~/.cobo/dev-qa.env (never printed).
"""
import json
import random
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "scripts" / "devqa"))
from devqa_env import base_url, optional, persona, run_sql  # noqa: E402

BASE = base_url()
OWN = optional("E2E_COMPANY_ID", "c_001")
TS = time.strftime("%Y%m%d%H%M%S")
CONTACT = f"qa.smoke.status.{TS}@cobo.test"
NOTE = f"ghi chu noi bo {TS}"
results = []


def call(method, path, token=None, body=None, raw=None):
    data = raw if raw is not None else (json.dumps(body).encode() if body is not None else None)
    req = urllib.request.Request(BASE + path, method=method, data=data, headers={"Content-Type": "application/json"})
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=40) as r:
            return r.status, json.loads(r.read() or b"{}")
    except urllib.error.HTTPError as e:
        try:
            return e.code, json.loads(e.read() or b"{}")
        except Exception:
            return e.code, {}


def record(ok, name, detail=""):
    results.append((bool(ok), name, str(detail)[:240]))


def err_code(body):
    return ((body or {}).get("error") or {}).get("code", "")


def field_errors(body):
    return (((body or {}).get("error") or {}).get("details") or {}).get("field_errors") or {}


def login(p, company):
    email, password = persona(p)
    st, body = call("POST", "/api/v1/auth/login", body={"email": email, "password": password})
    if st != 200:
        raise SystemExit(f"login failed for persona {p}: HTTP {st}")
    data = body.get("data") or body
    sess, ctx = data.get("session", {}), data.get("current_context") or {}
    if sess.get("access_token") and (ctx.get("company_id") == company or not sess.get("pre_company_token")):
        return sess["access_token"]
    st, body = call("POST", "/api/v1/auth/select-company", sess.get("pre_company_token") or sess.get("access_token"), {"company_id": company})
    if st != 200:
        raise SystemExit(f"select-company failed for persona {p}: HTTP {st}")
    return (body.get("data") or body)["access_token"]


def main():
    cms = login("CMS", OWN)
    ent = login("ENT", OWN)
    name = f"QA Smoke <b>Status</b> {TS}"
    tax = "09" + "".join(random.choice("0123456789") for _ in range(8))

    st, body = call("POST", "/api/v1/platform/cms/admin/companies", cms, {"company_name": name, "tax_code": "123"})
    record(st == 400 and "tax_code" in field_errors(body), "create: invalid tax code -> 400 field_errors", st)
    st, body = call("POST", "/api/v1/platform/cms/admin/companies", cms, {"company_name": name, "tax_code": tax, "contact_email": CONTACT})
    record(st == 201, "create disposable company", st)
    cid = ((body or {}).get("data") or {}).get("company_id")
    if not cid:
        raise SystemExit("company not created")
    code = (body.get("data") or {}).get("company_code")
    st, body = call("POST", "/api/v1/platform/cms/admin/companies", cms, {"company_name": name + " dup", "tax_code": tax})
    record(st == 409 and "tax_code" in field_errors(body), "create: duplicate tax code -> 409 (was 500)", st)

    path = f"/api/v1/platform/cms/admin/companies/{cid}"
    st, body = call("GET", path, cms)
    rev1 = (body.get("data") or {}).get("revision", "")
    record(st == 200 and rev1.startswith("co1:"), "detail has revision", rev1[:12])
    st, body = call("PATCH", path, cms, {"phone": "abc"})
    record(st == 400 and "phone" in field_errors(body), "patch: invalid phone -> 400 field_errors", st)
    st, body = call("PATCH", path, cms, {"address": "12 Lý Thường Kiệt", "expected_revision": rev1})
    rev2 = (body.get("data") or {}).get("revision", "")
    record(st == 200 and rev2 and rev2 != rev1, "patch with fresh revision -> 200, new revision", st)
    st, body = call("PATCH", path, cms, {"address": "Huế", "expected_revision": rev1})
    record(st == 409 and err_code(body) == "STALE_ETAG", "patch with stale revision -> 409 STALE_ETAG", st)

    st, body = call("POST", path + "/suspend", cms)
    record(st == 400 and "reason_code" in field_errors(body), "suspend without reason -> 400 (D-01)", st)
    st, body = call("POST", path + "/suspend", cms, {"reason_code": "OTHER", "reason_note": NOTE})
    change = (body.get("meta") or {}).get("status_change") or {}
    record(st == 200 and change.get("changed") and change.get("from") == "active" and change.get("to") == "suspended" and change.get("notified_count") == 1,
           "suspend -> 200, status_change, 1 email (contact fallback)", change)
    st, body = call("POST", path + "/suspend", cms, {"reason_code": "SECURITY"})
    record(st == 200 and ((body.get("meta") or {}).get("status_change") or {}).get("changed") is False, "suspend again -> no-op", st)
    st, body = call("POST", path + "/deactivate", cms, {"reason_code": "CONTRACT_ENDED", "confirm_company_code": "WRONG"})
    record(st == 400 and "confirm_company_code" in field_errors(body), "deactivate wrong code -> 400", st)
    st, body = call("POST", path + "/deactivate", cms, {"reason_code": "CONTRACT_ENDED", "confirm_company_code": code.lower()})
    record(st == 200 and (body.get("data") or {}).get("status") == "inactive", "deactivate with code (case-insensitive) -> inactive", st)
    st, body = call("POST", path + "/suspend", cms, {"reason_code": "SECURITY"})
    record(st == 409 and err_code(body) == "INVALID_STATE_TRANSITION", "inactive -> suspend -> 409 INVALID_STATE_TRANSITION", st)
    st, body = call("POST", path + "/activate", cms, {"reason_code": "PAYMENT_RECEIVED"})
    record(st == 200 and (body.get("data") or {}).get("status") == "active", "activate -> active", st)
    big = json.dumps({"reason_code": "OTHER", "reason_note": "x" * (20 << 10)}).encode()
    st, _ = call("POST", path + "/suspend", cms, raw=big)
    record(st == 413, "status body > 16 KiB -> 413", st)

    st, body = call("GET", path + "/history?limit=20", cms)
    items = (body.get("data") or {}).get("items") or []
    actions = [i.get("action") for i in items]
    record(st == 200 and actions.count("cms.company.status_change") == 3 and "cms.company.update" in actions and "cms.company.create" in actions,
           "history lists create, update, 3 status changes", actions)
    suspend_item = next((i for i in items if i.get("reason_code") == "OTHER"), {})
    record(suspend_item.get("reason_note") == NOTE and (suspend_item.get("actor") or {}).get("display_name"), "history: reason note + actor name", suspend_item.get("actor"))

    rows = run_sql(f"SELECT action, metadata_json FROM audit_logs WHERE company_id = '{cid}' ORDER BY occurred_at")
    create_meta = next((r.split("\t", 1)[1] for r in rows if r.startswith("cms.company.create")), "")
    record(CONTACT not in create_meta and "@" in create_meta, "audit (MySQL): contact email masked in create metadata", create_meta[:120])
    record(len(rows) >= 5, "audit rows written for every mutation", len(rows))

    st, _ = call("POST", path + "/suspend", ent, {"reason_code": "SECURITY"})
    record(st == 403, "enterprise admin cannot suspend -> 403", st)
    st, _ = call("GET", path + "/history", ent)
    record(st == 403, "enterprise admin cannot read history -> 403", st)

    # DEV sends through a real SMTP relay (Mailpit only holds old mail): check the durable
    # email_notifications rows instead. HTML escaping is covered by email_renderer_html_test.go.
    rows = []
    for _ in range(18):
        rows = run_sql(
            "SELECT status, variables_json_sanitized FROM email_notifications "
            f"WHERE recipient_email = '{CONTACT}' AND template_key = 'company.status_changed' ORDER BY created_at"
        )
        if len(rows) >= 3 and all(r.startswith("sent") for r in rows):
            break
        time.sleep(5)
    record(len(rows) == 3 and all(r.startswith("sent") for r in rows), "3 status emails sent (suspend, deactivate, activate)", [r.split("\t", 1)[0] for r in rows])
    record(rows and all(NOTE not in r for r in rows), "emails carry no internal note", "")

    # Leave the disposable company inactive (out of the way).
    st, _ = call("POST", path + "/deactivate", cms, {"reason_code": "OTHER", "reason_note": "qa smoke cleanup", "confirm_company_code": code})
    record(st == 200, "cleanup: disposable company deactivated", st)

    out = Path(__file__).with_suffix(".out")
    lines = [f"{'PASS' if ok else 'FAIL'}  {n}  {d}" for ok, n, d in results]
    lines.append(f"\n{sum(ok for ok, _, _ in results)}/{len(results)} PASS  (company {cid})")
    out.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print("\n".join(lines))
    return 0 if all(ok for ok, _, _ in results) else 1


if __name__ == "__main__":
    sys.exit(main())
