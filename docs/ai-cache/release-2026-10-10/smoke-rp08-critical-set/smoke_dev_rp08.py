"""DEV smoke for RP-08: removing a high-tier permission needs approval on the remove routes too.

QA company only (E2E_QA_COMPANY_ID): MEMBER gets a direct grant of workflow.step.override (a
HighRisk grant-policy code that was not in the narrow critical set) via qa_rw. ENT removes it:
- fixed binary: 202 APPROVAL_ROUTED, the grant stays until a second admin approves;
- old binary: 200, removed at once.
Cleanup cancels the approval, revokes the fixture grant and checks nothing is left pending.
"""
import json
import sys
import urllib.error
import urllib.request
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[4] / "scripts" / "devqa"))
from devqa_env import base_url, persona, require, run_sql  # noqa: E402

BASE = base_url()
QA = require("E2E_QA_COMPANY_ID")
A = "/api/v1/admin"
CODE = "workflow.step.override"
results = []


def call(method, path, token=None, body=None):
    req = urllib.request.Request(BASE + path, method=method, data=json.dumps(body).encode() if body is not None else None,
                                 headers={"Content-Type": "application/json"})
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=25) as r:
            return r.status, r.read()
    except urllib.error.HTTPError as e:
        return e.code, e.read()


def bodyj(raw):
    try:
        d = json.loads(raw)
    except Exception:
        return {}
    return d["data"] if isinstance(d, dict) and d.get("data") is not None else d


def code_of(raw):
    try:
        e = json.loads(raw).get("error") or {}
        return f"{e.get('code', '')} {e.get('message', '')}".strip()
    except Exception:
        return ""


def record(ok, name, detail=""):
    results.append((bool(ok), name, str(detail)[:200]))


def login(p, company):
    email, password = persona(p)
    st, raw = call("POST", "/api/v1/auth/login", body={"email": email, "password": password})
    if st != 200:
        raise SystemExit(f"login failed for persona {p}: HTTP {st}")
    body = bodyj(raw)
    sess, ctx = body.get("session", {}), body.get("current_context") or {}
    mids = {m["company_id"]: m["membership_id"] for m in body.get("memberships", [])}
    if sess.get("access_token") and ctx.get("company_id") == company:
        return sess["access_token"], ctx.get("membership_id")
    st, raw = call("POST", "/api/v1/auth/select-company", sess.get("pre_company_token") or sess.get("access_token"), {"company_id": company})
    if st != 200:
        raise SystemExit(f"select-company failed for persona {p}: HTTP {st}")
    return bodyj(raw)["access_token"], mids.get(company)


def q(v):
    return "'" + str(v).replace("'", "''") + "'"


def grant_active(mid):
    return run_sql(f"SELECT COUNT(*) FROM membership_direct_permissions WHERE membership_id={q(mid)} AND permission_code={q(CODE)} AND revoked_at IS NULL")


def pending():
    return run_sql(f"SELECT id FROM pending_admin_changes WHERE company_id={q(QA)} AND status='pending'")


def report():
    print()
    for ok, name, detail in results:
        print(f"{'PASS' if ok else 'FAIL'}  {name}  {detail}")
    print(f"\n{sum(r[0] for r in results)}/{len(results)} passed")


def main():
    ent, _ = login("ENT", QA)
    _, m_mem = login("MEMBER", QA)
    record(pending() == [], "fixture: no approval pending in the QA company", pending())
    try:
        run_sql("INSERT INTO membership_direct_permissions (membership_id, company_id, permission_code, granted_by) "
                f"VALUES ({q(m_mem)}, {q(QA)}, {q(CODE)}, 'qa_smoke_rp08') "
                "ON DUPLICATE KEY UPDATE revoked_at=NULL, revoked_by=NULL, granted_by='qa_smoke_rp08'", write=True)
        record(grant_active(m_mem) == ["1"], "fixture: MEMBER holds workflow.step.override directly", "")
        st, raw = call("DELETE", f"{A}/memberships/{m_mem}/permissions/{CODE}", ent)
        record(st == 202, "RP-08 high-tier removal is queued (202)", f"{st} {code_of(raw)}")
        record(grant_active(m_mem) == ["1"], "RP-08 grant still active until approved", grant_active(m_mem))
        aid = bodyj(raw).get("approval_id") if st == 202 else None
        if aid:
            st, raw = call("POST", f"{A}/config-approvals/{aid}/cancel", ent)
            record(st == 200, "cleanup: queued removal cancelled", f"{st} {code_of(raw)}")
    finally:
        for aid in pending():
            call("POST", f"{A}/config-approvals/{aid}/cancel", ent)
        run_sql("UPDATE membership_direct_permissions SET revoked_at=CURRENT_TIMESTAMP, revoked_by='qa_smoke_rp08' "
                f"WHERE membership_id={q(m_mem)} AND permission_code={q(CODE)} AND revoked_at IS NULL", write=True)
        call("PATCH", f"{A}/memberships/{m_mem}", ent, {"status": "active"})  # drop cached access
        record(grant_active(m_mem) == ["0"], "cleanup: fixture grant revoked", grant_active(m_mem))
        record(pending() == [], "cleanup: nothing left pending", pending())


if __name__ == "__main__":
    try:
        main()
    finally:
        report()
    sys.exit(0 if results and all(r[0] for r in results) else 1)
