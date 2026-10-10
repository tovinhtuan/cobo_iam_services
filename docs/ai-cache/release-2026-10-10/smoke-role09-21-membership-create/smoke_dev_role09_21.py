"""DEV smoke for ROLE-09 (new membership status allowlist) and ROLE-21 (role validated before a
platform assign-company writes anything).

QA data only:
- ROLE-09: ENT (tenant admin, E2E_COMPANY_ID) creates a membership for the CMS persona's user
  with status "suspended" -> 400 before any write (that user is already a member, so the old
  binary answered with a conflict / error instead).
- ROLE-21: the CMS persona (platform operator) assigns its own user to the QA company with a role
  of another company -> rejected, and no membership is created in the QA company.
- Positive: the same assignment with the QA company's user_thuong role works; the membership is
  deleted afterwards by ENT (owner of the QA company).
"""
import json
import sys
import urllib.error
import urllib.request
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[4] / "scripts" / "devqa"))
from devqa_env import base_url, optional, persona, require, run_sql  # noqa: E402

BASE = base_url()
OWN = optional("E2E_COMPANY_ID", "c_001")
QA = require("E2E_QA_COMPANY_ID")
ADMIN_ROLE_C002 = "r0000001-0001-4000-8000-000000000018"
CMS_USER = "u_qa_persona_cms"
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
    if sess.get("access_token") and ctx.get("company_id") == company:
        return sess["access_token"]
    st, raw = call("POST", "/api/v1/auth/select-company", sess.get("pre_company_token") or sess.get("access_token"), {"company_id": company})
    if st != 200:
        raise SystemExit(f"select-company failed for persona {p}: HTTP {st}")
    return bodyj(raw)["access_token"]


def q(v):
    return "'" + str(v).replace("'", "''") + "'"


def cms_in_qa():
    return run_sql(f"SELECT membership_id FROM memberships WHERE user_id={q(CMS_USER)} AND company_id={q(QA)}")


def report():
    print()
    for ok, name, detail in results:
        print(f"{'PASS' if ok else 'FAIL'}  {name}  {detail}")
    print(f"\n{sum(r[0] for r in results)}/{len(results)} passed")


def main():
    ent = login("ENT", OWN)
    ent_qa = login("ENT", QA)
    cms = login("CMS", OWN)
    before_own = run_sql(f"SELECT COUNT(*) FROM memberships WHERE company_id={q(OWN)}")
    record(cms_in_qa() == [], "fixture: CMS user is not a member of the QA company", cms_in_qa())
    # The platform assign route also needs admin.membership.invite in the operator's company:
    # grant it to the CMS persona for the run (qa_rw), then drop the company's cached access.
    run_sql("INSERT INTO membership_direct_permissions (membership_id, company_id, permission_code, granted_by) "
            f"VALUES ('m_qa_cms_c001', {q(OWN)}, 'admin.membership.invite', 'qa_smoke_role21') "
            "ON DUPLICATE KEY UPDATE revoked_at=NULL, revoked_by=NULL, granted_by='qa_smoke_role21'", write=True)
    call("PATCH", "/api/v1/admin/memberships/m_qa_member_c001", ent, {"status": "active"})
    try:
        # ROLE-09
        st, raw = call("POST", "/api/v1/admin/memberships", ent, {"user_id": CMS_USER, "status": "suspended"})
        record(st == 400 and code_of(raw).startswith("INVALID_REQUEST"), "ROLE-09 status 'suspended' rejected before any write",
               f"{st} {code_of(raw)}")
        record(run_sql(f"SELECT COUNT(*) FROM memberships WHERE company_id={q(OWN)}") == before_own, "ROLE-09 no membership written", "")

        # ROLE-21: role of another company
        st, raw = call("POST", f"/api/v1/platform/cms/admin/users/{CMS_USER}/assign-company", cms,
                       {"company_id": QA, "role_id": ADMIN_ROLE_C002})
        record(400 <= st < 500, "ROLE-21 role of another company rejected", f"{st} {code_of(raw)}")
        record(cms_in_qa() == [], "ROLE-21 no membership created in the QA company", cms_in_qa())
        for mid in cms_in_qa():  # old binary: undo the half-done assignment
            call("DELETE", f"/api/v1/admin/memberships/{mid}", ent_qa)

        # positive: a role of the target company works
        role = run_sql(f"SELECT role_id FROM roles WHERE company_id={q(QA)} AND role_code='user_thuong'")[0]
        st, raw = call("POST", f"/api/v1/platform/cms/admin/users/{CMS_USER}/assign-company", cms, {"company_id": QA, "role_id": role})
        mids = cms_in_qa()
        bound = run_sql(f"SELECT COUNT(*) FROM membership_roles WHERE membership_id={q(mids[0])} AND role_id={q(role)}") if mids else ["0"]
        record(st in (200, 201) and len(mids) == 1 and bound == ["1"], "platform assign with the company's own role works",
               f"{st} {code_of(raw)} roles={bound}")
    finally:
        run_sql("UPDATE membership_direct_permissions SET revoked_at=CURRENT_TIMESTAMP, revoked_by='qa_smoke_role21' "
                "WHERE membership_id='m_qa_cms_c001' AND revoked_at IS NULL AND granted_by='qa_smoke_role21'", write=True)
        call("PATCH", "/api/v1/admin/memberships/m_qa_member_c001", ent, {"status": "active"})
        left = run_sql("SELECT COUNT(*) FROM membership_direct_permissions WHERE membership_id='m_qa_cms_c001' "
                       "AND revoked_at IS NULL AND granted_by='qa_smoke_role21'")
        record(left == ["0"], "cleanup: fixture grant revoked", left)
        for mid in cms_in_qa():
            st, raw = call("DELETE", f"/api/v1/admin/memberships/{mid}", ent_qa)
            record(st in (200, 204), "cleanup: CMS membership in the QA company deleted", f"{st} {code_of(raw)}")
        record(cms_in_qa() == [], "cleanup: CMS user not a member of the QA company", cms_in_qa())


if __name__ == "__main__":
    try:
        main()
    finally:
        report()
    sys.exit(0 if results and all(r[0] for r in results) else 1)
