"""DEV smoke for ROLE-20: a non-operator cannot end a member's platform-operator status, whatever it
removes; removals that keep the status stay allowed.

QA company only (E2E_QA_COMPANY_ID). Fixture (reverted): MEMBER gets direct grants
platform.cms.view + system.settings (qa_rw), so it is an operator by definition; ENT (owner) is not.
- ENT removes MEMBER's direct system.settings -> 403 (old binary: 202, queued; approving it
  would have ended the operator status);
- ENT removes MEMBER's user_thuong role (status unaffected) -> 200; the fixture script restores it.
"""
import json
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path

DEVQA = Path(__file__).resolve().parents[4] / "scripts" / "devqa"
sys.path.insert(0, str(DEVQA))
from devqa_env import base_url, persona, require, run_sql  # noqa: E402

BASE = base_url()
QA = require("E2E_QA_COMPANY_ID")
A = "/api/v1/admin"
GRANTS = ("platform.cms.view", "system.settings")
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


def active_grants(mid):
    return sorted(run_sql(f"SELECT permission_code FROM membership_direct_permissions WHERE membership_id={q(mid)} "
                          f"AND revoked_at IS NULL AND granted_by='qa_smoke_role20'"))


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
    record(pending() == [], "fixture: no approval pending", pending())
    try:
        for code in GRANTS:
            run_sql("INSERT INTO membership_direct_permissions (membership_id, company_id, permission_code, granted_by) "
                    f"VALUES ({q(m_mem)}, {q(QA)}, {q(code)}, 'qa_smoke_role20') "
                    "ON DUPLICATE KEY UPDATE revoked_at=NULL, revoked_by=NULL, granted_by='qa_smoke_role20'", write=True)
        call("PATCH", f"{A}/memberships/{m_mem}", ent, {"status": "active"})  # drop cached access
        record(active_grants(m_mem) == sorted(GRANTS), "fixture: MEMBER is an operator (cms.view + system.settings)", active_grants(m_mem))

        st, raw = call("DELETE", f"{A}/memberships/{m_mem}/permissions/system.settings", ent)
        record(st == 403, "ROLE-20 owner cannot end MEMBER's operator status", f"{st} {code_of(raw)}")
        record("system.settings" in active_grants(m_mem) and pending() == [], "ROLE-20 nothing removed or queued", f"{active_grants(m_mem)} {pending()}")

        role = run_sql(f"SELECT r.role_id FROM membership_roles mr JOIN roles r ON r.role_id=mr.role_id "
                       f"WHERE mr.membership_id={q(m_mem)} AND r.role_code='user_thuong'")
        st, raw = call("DELETE", f"{A}/memberships/{m_mem}/roles/{role[0]}", ent)
        record(st == 200, "a removal that keeps operator status is allowed", f"{st} {code_of(raw)}")
    finally:
        for aid in pending():
            call("POST", f"{A}/config-approvals/{aid}/cancel", ent)
        run_sql("UPDATE membership_direct_permissions SET revoked_at=CURRENT_TIMESTAMP, revoked_by='qa_smoke_role20' "
                f"WHERE membership_id={q(m_mem)} AND revoked_at IS NULL AND granted_by='qa_smoke_role20'", write=True)
        subprocess.run([sys.executable, "-I", str(DEVQA / "provision_qa_company.py")], capture_output=True, timeout=300)
        call("PATCH", f"{A}/memberships/{m_mem}", ent, {"status": "active"})
        roles = run_sql(f"SELECT r.role_code FROM membership_roles mr JOIN roles r ON r.role_id=mr.role_id WHERE mr.membership_id={q(m_mem)}")
        record(active_grants(m_mem) == [] and pending() == [] and roles == ["user_thuong"], "cleanup: MEMBER back to user_thuong only",
               f"grants={active_grants(m_mem)} roles={roles}")


if __name__ == "__main__":
    try:
        main()
    finally:
        report()
    sys.exit(0 if results and all(r[0] for r in results) else 1)
