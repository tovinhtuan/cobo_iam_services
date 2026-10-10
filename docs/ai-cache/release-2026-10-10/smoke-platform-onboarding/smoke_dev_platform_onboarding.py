"""DEV smoke (analysis, no fix): onboarding a company through the platform CMS admin.

Observes what the current DEV binary does; every record is an observation with the expected value
from the code reading. QA data only; everything this run creates is purged at the end.

1. gates: CMS persona (cms_operator only) on the company / user routes; tenant ENT / MEMBER;
2. with a temporary admin.membership.invite grant for the CMS persona (qa_rw): create a company;
3. only with SMOKE_SEND_INVITE=1: invite the representative as admin_doanh_nghiep without that grant.
   DEV sends real email (SMTP smtp.gmail.com, not mailpit), so the token cannot be read back; run 1
   (smoke_dev_platform_onboarding.run1.out) did it once, to an undeliverable .test address;
4. password-based accounts: with company (default role), without company + assign-company as admin;
5. that admin signs in and tries owner-only actions (random password, never printed or stored);
6. the platform has no route to change a role in another company (tenant route with the CMS token);
7. deactivate the company and sign in again as its admin;
8. purge: the company and the users created here (root SQL, guarded by name / login prefix).
"""
import json
import secrets
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

DEVQA = Path(__file__).resolve().parents[4] / "scripts" / "devqa"
sys.path.insert(0, str(DEVQA))
from devqa_env import base_url, optional, persona, run_sql  # noqa: E402
from provision_qa_accounts import _run_root  # noqa: E402
from provision_qa_company import drop_access_cache  # noqa: E402

BASE = base_url()
OWN = optional("E2E_COMPANY_ID", "c_001")
P = "/api/v1/platform/cms/admin"
A = "/api/v1/admin"
TAG = "qa_smoke_onboard"
STAMP = time.strftime("%Y%m%d%H%M%S")
COMPANY_NAME = f"QA Platform Onboarding {STAMP} (smoke only)"
LOGIN_PREFIX = "qa.smoke.onboard."
results = []
created = {"company": None, "users": []}


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
    results.append((bool(ok), name, str(detail)[:260]))


def login_raw(email, password, company=None):
    st, raw = call("POST", "/api/v1/auth/login", body={"email": email, "password": password})
    if st != 200:
        return st, None, None, code_of(raw)
    body = bodyj(raw)
    sess, ctx = body.get("session", {}), body.get("current_context") or {}
    if sess.get("access_token") and (company is None or ctx.get("company_id") == company):
        return st, sess["access_token"], ctx.get("membership_id"), ""
    st, raw = call("POST", "/api/v1/auth/select-company", sess.get("pre_company_token") or sess.get("access_token"), {"company_id": company})
    if st != 200:
        return st, None, None, code_of(raw)
    data = bodyj(raw)
    return st, data.get("access_token"), (data.get("current_context") or {}).get("membership_id"), ""


def login(p, company):
    st, tok, mid, err = login_raw(*persona(p), company)
    if not tok:
        raise SystemExit(f"login failed for persona {p}: {st} {err}")
    return tok, mid


def q(v):
    return "'" + str(v).replace("'", "''") + "'"


def strong_password():
    return secrets.token_urlsafe(18) + "Aa1!"


def remember_user(user_id):
    if user_id and user_id not in created["users"]:
        created["users"].append(user_id)


def user_by_login(login_id):
    return (run_sql(f"SELECT user_id FROM users WHERE login_id={q(login_id)}") or [None])[0]


def membership(company_id, user_id):
    rows = run_sql("SELECT m.membership_id, m.membership_status, m.is_primary_admin, "
                   "IFNULL(GROUP_CONCAT(r.role_code ORDER BY r.role_code), '') FROM memberships m "
                   "LEFT JOIN membership_roles mr ON mr.membership_id=m.membership_id AND mr.status='active' "
                   f"LEFT JOIN roles r ON r.role_id=mr.role_id WHERE m.company_id={q(company_id)} AND m.user_id={q(user_id)} "
                   "GROUP BY m.membership_id")
    return rows[0].split("\t") if rows else None


def set_invite_grant(on):
    if on:
        run_sql("INSERT INTO membership_direct_permissions (membership_id, company_id, permission_code, granted_by) "
                f"VALUES ('m_qa_cms_c001', {q(OWN)}, 'admin.membership.invite', {q(TAG)}) "
                f"ON DUPLICATE KEY UPDATE revoked_at=NULL, revoked_by=NULL, granted_by={q(TAG)}", write=True)
    else:
        run_sql(f"UPDATE membership_direct_permissions SET revoked_at=CURRENT_TIMESTAMP, revoked_by={q(TAG)} "
                f"WHERE membership_id='m_qa_cms_c001' AND granted_by={q(TAG)} AND revoked_at IS NULL", write=True)
    drop_access_cache(OWN)


def purge():
    """Deletes the company and users this run created (and only those)."""
    cid = created["company"]
    users = [u for u in created["users"] if u]
    if cid:
        name = run_sql(f"SELECT company_name FROM companies WHERE company_id={q(cid)}")
        if name != [COMPANY_NAME]:
            record(False, "purge: company guard", f"refusing to delete {cid}: name {name}")
            return
    if users:
        logins = run_sql("SELECT login_id FROM users WHERE user_id IN (" + ",".join(q(u) for u in users) + ")")
        if not all(l.startswith(LOGIN_PREFIX) for l in logins):
            record(False, "purge: user guard", f"refusing to delete users with logins {logins}")
            return
    sql = ["SET FOREIGN_KEY_CHECKS=0;", "START TRANSACTION;"]
    if cid:
        c = q(cid)
        sql += [
            f"DELETE mr FROM membership_roles mr JOIN memberships m ON m.membership_id=mr.membership_id WHERE m.company_id={c};",
            f"DELETE dm FROM department_memberships dm JOIN memberships m ON m.membership_id=dm.membership_id WHERE m.company_id={c};",
            f"DELETE mt FROM membership_titles mt JOIN memberships m ON m.membership_id=mt.membership_id WHERE m.company_id={c};",
            f"DELETE rp FROM role_permissions rp JOIN roles r ON r.role_id=rp.role_id WHERE r.company_id={c};",
        ]
        tables = _run_root("SELECT c.table_name FROM information_schema.columns c JOIN information_schema.tables t "
                           "ON t.table_schema=c.table_schema AND t.table_name=c.table_name AND t.table_type='BASE TABLE' "
                           "WHERE c.table_schema='cobo_iam' AND c.column_name='company_id' AND c.table_name<>'companies';").split()
        sql += [f"DELETE FROM `{t}` WHERE company_id={c};" for t in tables]
        sql += [f"DELETE FROM companies WHERE company_id={c};"]
    if users:
        ids = ",".join(q(u) for u in users)
        tables = _run_root("SELECT c.table_name FROM information_schema.columns c JOIN information_schema.tables t "
                           "ON t.table_schema=c.table_schema AND t.table_name=c.table_name AND t.table_type='BASE TABLE' "
                           "WHERE c.table_schema='cobo_iam' AND c.column_name='user_id' AND c.table_name<>'users';").split()
        sql += [f"DELETE FROM `{t}` WHERE user_id IN ({ids});" for t in tables]
        sql += [f"DELETE FROM users WHERE user_id IN ({ids});"]
    sql += ["COMMIT;", "SET FOREIGN_KEY_CHECKS=1;"]
    _run_root("\n".join(sql))
    left_c = run_sql(f"SELECT COUNT(*) FROM companies WHERE company_id={q(cid)}") if cid else ["0"]
    left_u = run_sql("SELECT COUNT(*) FROM users WHERE user_id IN (" + ",".join(q(u) for u in users) + ")") if users else ["0"]
    left_m = run_sql(f"SELECT COUNT(*) FROM memberships WHERE company_id={q(cid)}") if cid else ["0"]
    record(left_c == ["0"] and left_u == ["0"] and left_m == ["0"], "purge: company, memberships and users removed",
           f"company={left_c} users={left_u} memberships={left_m}")


def admin_checks(tok, mid, label):
    st, raw = call("GET", f"{A}/memberships/{mid}/permissions", tok)
    record(st == 200, f"{label} can use the tenant admin API", f"{st} {code_of(raw)}")
    st, raw = call("POST", f"{A}/company/transfer-ownership", tok, {"target_membership_id": mid})
    record(st == 403, f"{label} cannot transfer ownership (not the primary admin)", f"{st} {code_of(raw)}")
    st, raw = call("POST", f"{A}/memberships/{mid}/permissions", tok, {"permission_code": "admin.membership.invite"})
    record(st == 403, f"{label} cannot grant admin.membership.invite (reserved to the owner)", f"{st} {code_of(raw)}")


def report():
    print()
    for ok, name, detail in results:
        print(f"{'PASS' if ok else 'FAIL'}  {name}  {detail}")
    print(f"\n{sum(r[0] for r in results)}/{len(results)} as expected")


def main():
    cms, _ = login("CMS", OWN)
    ent, _ = login("ENT", OWN)
    member, _ = login("MEMBER", OWN)

    # 1. gates (CMS persona as provisioned: cms_operator only)
    st, raw = call("POST", f"{P}/companies", cms, {"company_name": COMPANY_NAME})
    if st in (200, 201):
        created["company"] = bodyj(raw).get("company_id")
    record(st == 403, "gate: cms_operator only cannot create a company", f"{st} {code_of(raw)}")
    st, raw = call("GET", f"{P}/companies?limit=1", cms)
    record(st == 403, "gate: cms_operator only cannot list companies", f"{st} {code_of(raw)}")
    st, raw = call("GET", f"{P}/users?limit=1", cms)
    record(st == 200, "gate: cms_operator only can list users", f"{st} {code_of(raw)}")
    probe = f"{LOGIN_PREFIX}probe.{STAMP}@cobo.test"
    st, raw = call("POST", f"{P}/users", cms, {"login_id": probe, "password": strong_password(), "full_name": "QA Onboard Probe"})
    remember_user(bodyj(raw).get("user_id") if st in (200, 201) else None)
    record(st == 403, "gate: cms_operator only cannot create a password account", f"{st} {code_of(raw)}")
    st, raw = call("POST", f"{P}/companies", ent, {"company_name": COMPANY_NAME})
    record(st == 403, "gate: tenant admin (no platform.cms.view) gets 403 on platform routes", f"{st} {code_of(raw)}")
    st, raw = call("GET", f"{P}/companies?limit=1", member)
    record(st == 403, "gate: plain member gets 403 on platform routes", f"{st} {code_of(raw)}")

    try:
        # 2. create the company with a temporary invite grant
        set_invite_grant(True)
        if not created["company"]:
            st, raw = call("POST", f"{P}/companies", cms, {"company_name": COMPANY_NAME, "contact_email": "qa.smoke@cobo.test"})
            created["company"] = bodyj(raw).get("company_id") if st in (200, 201) else None
            record(bool(created["company"]), "create company (with admin.membership.invite)", f"{st} {code_of(raw)}")
        cid = created["company"]
        if not cid:
            return
        info = run_sql(f"SELECT status, verification_status FROM companies WHERE company_id={q(cid)}")
        roles = run_sql(f"SELECT GROUP_CONCAT(role_code ORDER BY role_code) FROM roles WHERE company_id={q(cid)}")
        members = run_sql(f"SELECT COUNT(*) FROM memberships WHERE company_id={q(cid)}")
        record(members == ["0"], "new company: default roles, no member, no owner", f"status={info} roles={roles} members={members}")

        # 3. invite: sends a real email on DEV, so only on request
        if optional("SMOKE_SEND_INVITE", "") == "1":
            set_invite_grant(False)
            rep = f"{LOGIN_PREFIX}rep.{STAMP}@cobo.test"
            st, raw = call("POST", f"{P}/users/invite", cms, {"email": rep, "full_name": "QA Onboard Representative",
                                                              "company_id": cid, "role_code": "admin_doanh_nghiep"})
            rep_user = bodyj(raw).get("user_id") if st in (200, 201) else None
            remember_user(rep_user or user_by_login(rep))
            rep_m = membership(cid, rep_user) if rep_user else None
            record(st in (200, 201) and rep_m and rep_m[3] == "admin_doanh_nghiep" and rep_m[2] == "0",
                   "invite representative as admin (rbac.manage only): admin role, not owner", f"{st} {code_of(raw)} membership={rep_m}")
            set_invite_grant(True)

        # 4. password-based accounts
        staff = f"{LOGIN_PREFIX}staff.{STAMP}@cobo.test"
        st, raw = call("POST", f"{P}/users", cms, {"login_id": staff, "password": strong_password(), "full_name": "QA Onboard Staff",
                                                   "company_id": cid})
        staff_user = bodyj(raw).get("user_id") if st in (200, 201) else None
        remember_user(staff_user or user_by_login(staff))
        sm = membership(cid, staff_user) if staff_user else None
        record(st in (200, 201) and sm is not None, "create password account in the company (no role field)",
               f"{st} {code_of(raw)} membership={sm}")
        admin2, pw2 = f"{LOGIN_PREFIX}admin2.{STAMP}@cobo.test", strong_password()
        st, raw = call("POST", f"{P}/users", cms, {"login_id": admin2, "password": pw2, "full_name": "QA Onboard Admin"})
        a2_user = bodyj(raw).get("user_id") if st in (200, 201) else None
        remember_user(a2_user or user_by_login(admin2))
        record(st in (200, 201) and a2_user, "create password account without a company", f"{st} {code_of(raw)}")
        if a2_user:
            st, raw = call("POST", f"{P}/users/{a2_user}/assign-company", cms, {"company_id": cid, "role_code": "admin_doanh_nghiep"})
            a2m = membership(cid, a2_user)
            record(st in (200, 201) and a2m and a2m[3] == "admin_doanh_nghiep" and a2m[2] == "0",
                   "assign that account to the company as admin (not owner)", f"{st} {code_of(raw)} membership={a2m}")
            # 5. the company's admin signs in and tries owner-only actions
            st, tok, mid, err = login_raw(admin2, pw2, cid)
            record(bool(tok), "the company's admin signs in", f"{st} {err}")
            if tok:
                admin_checks(tok, mid, "company admin (not owner)")
                created["admin2"] = (tok, mid)

        # 6. platform changing a role in another company
        if sm:
            admin_role = run_sql(f"SELECT role_id FROM roles WHERE company_id={q(cid)} AND role_code='admin_doanh_nghiep'")[0]
            st, raw = call("PUT", f"{A}/memberships/{sm[0]}/primary-role", cms, {"role_id": admin_role})
            record(st in (403, 404) and membership(cid, staff_user)[3] == sm[3],
                   "CMS token cannot change a role in another company (tenant routes are token-scoped)", f"{st} {code_of(raw)}")

        # 7. deactivate the company, then its admin signs in again
        set_invite_grant(False)
        st, raw = call("POST", f"{P}/companies/{cid}/deactivate", cms, {"reason": "qa smoke"})
        record(st == 403, "gate: cms_operator only cannot deactivate a company", f"{st} {code_of(raw)}")
        set_invite_grant(True)
        st, raw = call("POST", f"{P}/companies/{cid}/deactivate", cms, {"reason": "qa smoke"})
        record(st == 200, "deactivate the company (with admin.membership.invite)", f"{st} {code_of(raw)}")
        status = run_sql(f"SELECT status FROM companies WHERE company_id={q(cid)}")
        record(status != ["active"], "company status after deactivate", status)
        if created.get("admin2"):
            old_tok, mid = created["admin2"]
            st, raw = call("GET", f"{A}/memberships/{mid}/permissions", old_tok)
            record(st in (401, 403), "deactivated company: the admin's existing session is cut off", f"{st} {code_of(raw)}")
            st, tok, _, err = login_raw(admin2, pw2, cid)
            record(not tok, "deactivated company: its admin cannot sign in to it", f"{st} {err}")
            if tok:
                st, raw = call("GET", f"{A}/memberships/{mid}/permissions", tok)
                record(st in (401, 403), "deactivated company: a fresh session cannot use the admin API", f"{st} {code_of(raw)}")
                st, raw = call("GET", "/api/v1/me/authorized-companies", tok)
                listed = [c.get("company_id") for c in (bodyj(raw).get("items") or bodyj(raw).get("companies") or [])] \
                    if isinstance(bodyj(raw), dict) else [c.get("company_id") for c in bodyj(raw)]
                record(cid not in listed, "deactivated company: not offered in authorized companies", f"{st} listed={cid in listed}")
    finally:
        set_invite_grant(False)
        left = run_sql(f"SELECT COUNT(*) FROM membership_direct_permissions WHERE membership_id='m_qa_cms_c001' "
                       f"AND granted_by={q(TAG)} AND revoked_at IS NULL")
        record(left == ["0"], "cleanup: temporary grant revoked", left)
        purge()


if __name__ == "__main__":
    try:
        main()
    finally:
        report()
