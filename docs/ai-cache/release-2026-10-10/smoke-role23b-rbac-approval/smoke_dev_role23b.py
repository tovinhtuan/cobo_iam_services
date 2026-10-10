"""DEV smoke for ROLE-23 (role level): an RBAC approval cannot take rbac.manage from the last admin.

QA company only (E2E_QA_COMPANY_ID), fixture `provision_qa_company.py --no-primary` plus direct
grants through qa_rw (granted_by=qa_smoke_role23b, revoked at the end):
- MEMBER holds rbac.manage directly (an admin through a removable grant);
- ENT2 holds system.settings directly and is demoted to user_thuong: an approver, not an admin.
Cases:
- submit: MEMBER, the only admin, asks to remove its own rbac.manage -> 409 (old binary: 202, and
  ENT2's approval left the company with no admin);
- approve: queued while ENT was still an admin, ENT demoted afterwards -> approving is 409 (old
  binary: 200, no admin left);
- positive: ENT stays an admin -> queued (202) and approved (200), MEMBER loses rbac.manage;
- the requester cannot approve its own request (403); another company's token gets 404.
"""
import json
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path

DEVQA = Path(__file__).resolve().parents[4] / "scripts" / "devqa"
sys.path.insert(0, str(DEVQA))
from devqa_env import base_url, optional, persona, require, run_sql  # noqa: E402
from provision_qa_company import drop_access_cache  # noqa: E402

BASE = base_url()
QA = require("E2E_QA_COMPANY_ID")
OWN = optional("E2E_COMPANY_ID", "c_001")
A = "/api/v1/admin"
LAST = "last_admin_role_change_blocked"
TAG = "qa_smoke_role23b"
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
    results.append((bool(ok), name, str(detail)[:240]))


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


def state():
    return run_sql("SELECT m.membership_id, m.membership_status, m.is_primary_admin, "
                   "IFNULL(GROUP_CONCAT(r.role_code ORDER BY r.role_code), '') FROM memberships m "
                   "LEFT JOIN membership_roles mr ON mr.membership_id=m.membership_id AND mr.status='active' "
                   f"LEFT JOIN roles r ON r.role_id=mr.role_id WHERE m.company_id={q(QA)} "
                   "GROUP BY m.membership_id ORDER BY m.membership_id")


def admins():
    """Active members holding rbac.manage through a role or a direct grant."""
    return run_sql("SELECT DISTINCT m.membership_id FROM memberships m "
                   "LEFT JOIN membership_roles mr ON mr.membership_id=m.membership_id AND mr.status='active' "
                   "LEFT JOIN role_permissions rp ON rp.role_id=mr.role_id "
                   "LEFT JOIN permissions p ON p.permission_id=rp.permission_id AND p.permission_code='rbac.manage' "
                   "LEFT JOIN membership_direct_permissions d ON d.membership_id=m.membership_id "
                   "AND d.permission_code='rbac.manage' AND d.revoked_at IS NULL "
                   f"WHERE m.company_id={q(QA)} AND m.membership_status='active' "
                   "AND (p.permission_id IS NOT NULL OR d.membership_id IS NOT NULL) ORDER BY m.membership_id")


def pending():
    return run_sql(f"SELECT id FROM pending_admin_changes WHERE company_id={q(QA)} AND status='pending'")


def fixture_grants():
    return run_sql(f"SELECT membership_id, permission_code FROM membership_direct_permissions WHERE company_id={q(QA)} "
                   f"AND granted_by={q(TAG)} AND revoked_at IS NULL ORDER BY membership_id")


def grant(mid, code):
    run_sql("INSERT INTO membership_direct_permissions (membership_id, company_id, permission_code, granted_by) "
            f"VALUES ({q(mid)}, {q(QA)}, {q(code)}, {q(TAG)}) "
            f"ON DUPLICATE KEY UPDATE revoked_at=NULL, revoked_by=NULL, granted_by={q(TAG)}", write=True)


def revoke_fixture_grants():
    run_sql(f"UPDATE membership_direct_permissions SET revoked_at=CURRENT_TIMESTAMP, revoked_by={q(TAG)} "
            f"WHERE company_id={q(QA)} AND granted_by={q(TAG)} AND revoked_at IS NULL", write=True)


def provision(no_primary):
    args = [sys.executable, "-I", str(DEVQA / "provision_qa_company.py")] + (["--no-primary"] if no_primary else [])
    return subprocess.run(args, capture_output=True, timeout=300).returncode


def user_role():
    return run_sql(f"SELECT role_id FROM roles WHERE company_id={q(QA)} AND role_code='user_thuong'")[0]


def cancel_pending():
    for p in ("MEMBER", "ENT2"):
        if not pending():
            return
        try:
            tok, _ = login(p, QA)
        except SystemExit:
            continue
        for aid in pending():
            call("POST", f"{A}/config-approvals/{aid}/cancel", tok)


def setup(demote):
    """No primary admin; MEMBER admin by direct rbac.manage; ENT2 approver by system.settings.
    MEMBER demotes the listed personas to user_thuong. Returns tokens and membership ids."""
    cancel_pending()
    revoke_fixture_grants()
    provision(True)
    t = {p: login(p, QA) for p in ("ENT", "ENT2", "MEMBER")}
    grant(t["MEMBER"][1], "rbac.manage")
    grant(t["ENT2"][1], "system.settings")
    drop_access_cache(QA)
    for p in demote:
        st, raw = call("PUT", f"{A}/memberships/{t[p][1]}/primary-role", t["MEMBER"][0], {"role_id": user_role()})
        if st != 200:
            raise SystemExit(f"fixture: demoting {p} failed: {st} {code_of(raw)}")
    return t


def remove_own_rbac(t):
    return call("DELETE", f"{A}/memberships/{t['MEMBER'][1]}/permissions/rbac.manage", t["MEMBER"][0])


def approve(t, aid, p="ENT2"):
    return call("POST", f"{A}/config-approvals/{aid}/approve", t[p][0])


def report():
    print()
    for ok, name, detail in results:
        print(f"{'PASS' if ok else 'FAIL'}  {name}  {detail}")
    print(f"\n{sum(r[0] for r in results)}/{len(results)} passed")


def main():
    before = state()
    try:
        # submit: MEMBER is the only admin
        t = setup(["ENT", "ENT2"])
        m_mem = t["MEMBER"][1]
        record(admins() == [m_mem] and pending() == [], "fixture: MEMBER is the only admin, nothing pending", admins())
        st, raw = remove_own_rbac(t)
        record(st == 409 and code_of(raw).startswith(LAST) and admins() == [m_mem] and pending() == [],
               "submit: removing the last admin's rbac.manage is refused", f"{st} {code_of(raw)}")
        if st == 202:  # old binary: show what approving it does
            st2, raw2 = approve(t, bodyj(raw).get("approval_id"))
            record(False, "old binary: approving it", f"{st2} {code_of(raw2)} admins_after={admins()}")

        # approve: queued while ENT was an admin, ENT demoted before the approval
        t = setup(["ENT2"])
        m_mem, m_ent = t["MEMBER"][1], t["ENT"][1]
        record(sorted(admins()) == sorted([m_ent, m_mem]), "fixture: ENT and MEMBER are the admins", admins())
        st, raw = remove_own_rbac(t)
        aid = bodyj(raw).get("approval_id") if st == 202 else None
        record(st == 202 and aid, "approve: removal queued while ENT is still an admin", f"{st} {code_of(raw)}")
        st, raw = call("PUT", f"{A}/memberships/{m_ent}/primary-role", t["MEMBER"][0], {"role_id": user_role()})
        record(st == 200 and admins() == [m_mem], "approve: ENT demoted, MEMBER now the only admin", f"{st} {code_of(raw)}")
        if aid:
            st, raw = approve(t, aid, "MEMBER")
            record(st == 403, "requester cannot approve its own request", f"{st} {code_of(raw)}")
            other, _ = login("ENT", OWN)
            st, raw = call("POST", f"{A}/config-approvals/{aid}/approve", other)
            record(st in (403, 404), "another company's token cannot approve it", f"{st} {code_of(raw)}")
            st, raw = approve(t, aid)
            record(st == 409 and code_of(raw).startswith(LAST) and admins() == [m_mem],
                   "approve: approving it would leave no admin -> 409", f"{st} {code_of(raw)} admins_after={admins()}")

        # positive: ENT stays an admin
        t = setup(["ENT2"])
        m_mem, m_ent = t["MEMBER"][1], t["ENT"][1]
        st, raw = remove_own_rbac(t)
        aid = bodyj(raw).get("approval_id") if st == 202 else None
        record(st == 202 and aid, "positive: removal queued (another admin stays)", f"{st} {code_of(raw)}")
        if aid:
            st, raw = approve(t, aid)
            record(st == 200 and admins() == [m_ent], "positive: approved, MEMBER loses rbac.manage, ENT stays admin",
                   f"{st} {code_of(raw)} admins={admins()}")
    finally:
        cancel_pending()
        revoke_fixture_grants()
        rc = provision(False)
        record(rc == 0 and state() == before and fixture_grants() == [] and pending() == [],
               "cleanup: QA company back to its starting state, no fixture grant, nothing pending",
               f"state={state()} grants={fixture_grants()} pending={pending()}")


if __name__ == "__main__":
    try:
        main()
    finally:
        report()
    sys.exit(0 if results and all(r[0] for r in results) else 1)
