"""DEV smoke for the 2026-10-10 risk-review release: PR-A (2996f00, access revocation / cache) and
PR-B (c6b4ab6, admin escalation / cross-tenant).

QA data only: every write targets a QA persona membership (m_qa_*) and is either rejected by the
server or reverted in a finally block; the run ends by checking the QA memberships are back to
their starting state. Personas, base URL, SSH and DB users come from ~/.cobo/dev-qa.env through
scripts/devqa/devqa_env.py (see ../cobo_web_design/docs/ai-cache/dev-qa/README.md). Never prints
passwords or tokens.

Not covered here (would need writes on non-QA data, or no fixture on DEV): ROLE-05 (primary
admin is a real account), ROLE-02 (qa_rw cannot clean resource_scope_rules), ROLE-12 / BES-18
(no alert_channel_prefs rule on DEV). Those are covered by unit / MySQL integration tests.
"""
import json
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[4] / "scripts" / "devqa"))
from devqa_env import base_url, optional, persona, run_sql, ssh_cmd  # noqa: E402

BASE = base_url()
OWN = optional("E2E_COMPANY_ID", "c_001")
OTHER = optional("E2E_OTHER_COMPANY_ID", "c_002")
CMS_OPERATOR_ROLE_C001 = "r0000001-0001-4000-8000-000000000016"
ADMIN_ROLE_C002 = "r0000001-0001-4000-8000-000000000018"
A = "/api/v1/admin"
results = []


def call(method, path, token=None, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE + path, data=data, method=method)
    req.add_header("Content-Type", "application/json")
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
    return d["data"] if isinstance(d, dict) and "data" in d and d["data"] is not None else d


def code_of(raw):
    try:
        return (json.loads(raw).get("error") or {}).get("code", "")
    except Exception:
        return ""


def record(ok, name, detail=""):
    results.append((bool(ok), name, str(detail)[:160]))
    return bool(ok)


def check(name, method, path, token, expect, body=None, code=None):
    """expect: one status or a tuple of accepted statuses."""
    status, raw = call(method, path, token, body)
    accepted = expect if isinstance(expect, tuple) else (expect,)
    err = code_of(raw) if status >= 300 else ""
    record(status in accepted and (code is None or err == code), name, f"{method} {path} -> {status} {err}")
    return status, raw


def login(p, company):
    email, password = persona(p)
    status, raw = call("POST", "/api/v1/auth/login", body={"email": email, "password": password})
    if status != 200:
        raise SystemExit(f"login failed for persona {p}: HTTP {status} {code_of(raw)}")
    body = bodyj(raw)
    sess = body.get("session", {})
    mids = {m["company_id"]: m["membership_id"] for m in body.get("memberships", [])}
    ctx = body.get("current_context") or {}
    if ctx.get("company_id") and ctx.get("membership_id"):  # single-company login is auto-selected
        mids.setdefault(ctx["company_id"], ctx["membership_id"])
    if sess.get("access_token") and ctx.get("company_id") == company:
        return sess["access_token"], sess.get("refresh_token"), mids
    pre = sess.get("pre_company_token") or sess.get("access_token")
    status, raw = call("POST", "/api/v1/auth/select-company", pre, {"company_id": company})
    if status != 200:
        raise SystemExit(f"select-company {company} failed for persona {p}: HTTP {status} {code_of(raw)}")
    sel = bodyj(raw)
    return sel["access_token"], sel.get("refresh_token") or sess.get("refresh_token"), mids


def q(v):
    return "'" + str(v).replace("'", "''") + "'"


def qa_state(mids):
    rows = run_sql("SELECT membership_id, membership_status FROM memberships WHERE membership_id IN ("
                   + ",".join(q(m) for m in mids) + ") ORDER BY membership_id")
    return dict(r.split("\t") for r in rows)


def perms_of(token):
    st, raw = call("GET", "/api/v1/me/effective-access", token)
    body = bodyj(raw) if st == 200 else {}
    return st, (body.get("permissions") or []) if isinstance(body, dict) else []


def report():
    print()
    for ok, name, detail in results:
        print(f"{'PASS' if ok else 'FAIL'}  {name}  {detail}")
    print(f"\n{sum(r[0] for r in results)}/{len(results)} passed")


def main():
    ent, _, ent_m = login("ENT", OWN)
    mem, mem_ref, mem_m = login("MEMBER", OWN)
    cms, _, cms_m = login("CMS", OWN)
    _, _, usr_m = login("USER", OWN)
    m_ent, m_ent2, m_mem, m_cms = ent_m[OWN], login("ENT2", OWN)[2][OWN], mem_m[OWN], cms_m[OWN]
    m_usr_other = usr_m.get(OTHER)
    qa_mids = [m for m in (m_ent, m_ent2, m_mem, m_cms, m_usr_other) if m]
    if not all(m.startswith("m_qa_") for m in qa_mids):
        raise SystemExit(f"refusing to run: a persona is not a QA membership: {qa_mids}")
    before = qa_state(qa_mids)
    cms_bindings_before = run_sql(f"SELECT COUNT(*) FROM membership_roles WHERE membership_id={q(m_cms)} "
                                  f"AND role_id={q(CMS_OPERATOR_ROLE_C001)} AND status='active'")
    print(f"company={OWN} other={OTHER} qa memberships={qa_mids}")

    try:
        # --- happy path: ENT (tenant admin) -----------------------------------------------------
        st, perms = perms_of(ent)
        record(st == 200 and "rbac.manage" in perms, "ENT effective access has rbac.manage", f"{st} n={len(perms)}")
        check("ENT lists roles", "GET", f"{A}/roles", ent, 200)
        check("ENT lists company memberships", "GET", f"{A}/companies/{OWN}/memberships", ent, 200)
        st, raw = check("ENT invite-roles (ROLE-03: admin is unrestricted)", "GET", f"{A}/invite-roles", ent, 200)
        roles = bodyj(raw)
        roles = roles.get("items", roles) if isinstance(roles, dict) else roles
        # The picker only offers roles the invite accepts. c_001's admin_doanh_nghiep carries
        # disclosure_type.manage (module "cms"), which a tenant admin cannot hand out (ROLE-08),
        # so it is not offered there; ordinary roles are.
        codes = sorted({r.get("role_code") for r in roles or []})
        record("user_thuong" in codes, "ROLE-03 company admin picker offers grantable roles", codes)
        st, _ = call("GET", "/api/v1/platform/cms/admin/users?company_id=" + OWN, cms)
        record(st == 200, "CMS operator reaches platform CMS", st)

        # --- MEMBER is refused admin routes ------------------------------------------------------
        check("MEMBER cannot list roles", "GET", f"{A}/roles", mem, 403)
        check("MEMBER cannot list invite roles", "GET", f"{A}/invite-roles", mem, 403)
        check("MEMBER cannot deactivate a member", "PATCH", f"{A}/memberships/{m_ent2}", mem, 403, {"status": "inactive"})

        # --- other company is refused ------------------------------------------------------------
        check("ENT cannot list other company's memberships", "GET", f"{A}/companies/{OTHER}/memberships", ent, (403, 404))
        if m_usr_other:
            check("ENT cannot deactivate a member of the other company", "PATCH", f"{A}/memberships/{m_usr_other}", ent,
                  (403, 404), {"status": "inactive"})
            check("ENT cannot remove a role in the other company", "DELETE",
                  f"{A}/memberships/{m_usr_other}/roles/{ADMIN_ROLE_C002}", ent, (403, 404))

        # --- BES-12: membership status values ----------------------------------------------------
        check("BES-12 status 'suspended' rejected", "PATCH", f"{A}/memberships/{m_mem}", ent, 400, {"status": "suspended"},
              "INVALID_REQUEST")

        # --- BES-21 / ROLE-13: tenant admin cannot take platform access away --------------------
        check("BES-21 ENT cannot deactivate the platform operator", "PATCH", f"{A}/memberships/{m_cms}", ent, 403,
              {"status": "inactive"}, "PERMISSION_DENIED")
        check("BES-21 ENT cannot delete the platform operator", "DELETE", f"{A}/memberships/{m_cms}", ent, 403,
              None, "PERMISSION_DENIED")
        check("ROLE-13 ENT cannot remove the cms_operator role", "DELETE",
              f"{A}/memberships/{m_cms}/roles/{CMS_OPERATOR_ROLE_C001}", ent, 403, None, "PERMISSION_DENIED")
        check("ROLE-13 ENT cannot remove platform.cms.view", "DELETE",
              f"{A}/memberships/{m_cms}/permissions/platform.cms.view", ent, 403, None, "PERMISSION_DENIED")
        st, _ = call("GET", "/api/v1/platform/cms/admin/users?company_id=" + OWN, cms)
        record(st == 200, "CMS operator still reaches platform CMS", st)

        # --- H3 / H17 / CACHE-11: deactivation takes effect at once ------------------------------
        st, perms = perms_of(mem)  # also fills the effective-access cache
        record(st == 200 and len(perms) > 0, "H3 MEMBER has permissions before deactivation", f"{st} n={len(perms)}")
        try:
            check("H3 ENT deactivates MEMBER", "PATCH", f"{A}/memberships/{m_mem}", ent, 200, {"status": "inactive"})
            st, perms = perms_of(mem)
            record(st != 200 or len(perms) == 0, "H3/H17 inactive MEMBER loses permissions at once", f"{st} n={len(perms)}")
            row = run_sql(f"SELECT membership_status FROM memberships WHERE membership_id={q(m_mem)}")
            record(row == ["inactive"], "H3 DB shows MEMBER inactive", row)
            st, raw = call("POST", "/api/v1/auth/refresh", body={"refresh_token": mem_ref})
            record(st == 401 and code_of(raw) == "SESSION_EXPIRED", "H3 refresh refused for inactive MEMBER", f"{st} {code_of(raw)}")
        finally:
            st, _ = call("PATCH", f"{A}/memberships/{m_mem}", ent, {"status": "active"})
            record(st == 200, "restore: MEMBER reactivated", st)
        mem2, _, _ = login("MEMBER", OWN)
        st, perms = perms_of(mem2)
        record(st == 200 and len(perms) > 0, "H3 reactivated MEMBER has permissions again", f"{st} n={len(perms)}")

        # --- CACHE: v2 namespace in use ----------------------------------------------------------
        r = subprocess.run(ssh_cmd() + ["docker exec cobo-iam-redis redis-cli --scan --pattern 'cobo_iam:effective_access*:v2:*' | wc -l"],
                           capture_output=True, text=True, timeout=60)
        n = r.stdout.strip()
        record(r.returncode == 0 and n.isdigit() and int(n) > 0, "CACHE v2 keys written", n)
    finally:
        # --- QA data back to its starting state --------------------------------------------------
        after = qa_state(qa_mids)
        for mid, status in before.items():
            if after.get(mid) != status:
                call("PATCH", f"{A}/memberships/{mid}", ent, {"status": status})
        after = qa_state(qa_mids)
        record(after == before, "cleanup: QA memberships unchanged", f"before={before} after={after}")
        cms_bindings_after = run_sql(f"SELECT COUNT(*) FROM membership_roles WHERE membership_id={q(m_cms)} "
                                     f"AND role_id={q(CMS_OPERATOR_ROLE_C001)} AND status='active'")
        record(cms_bindings_after == cms_bindings_before, "cleanup: CMS persona keeps cms_operator role",
               f"{cms_bindings_before} -> {cms_bindings_after}")
        pending = run_sql(f"SELECT COUNT(*) FROM pending_admin_changes WHERE company_id={q(OWN)} AND status='pending'")
        record(pending == ["0"], "cleanup: no approval left pending", pending)


if __name__ == "__main__":
    try:
        main()
    finally:
        report()
    sys.exit(0 if results and all(r[0] for r in results) else 1)
