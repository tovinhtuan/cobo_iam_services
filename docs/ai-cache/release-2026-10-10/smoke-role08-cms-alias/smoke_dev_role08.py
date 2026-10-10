"""DEV smoke for ROLE-08 / RP-06: disclosure_type.manage (tenant-grantable) does not open the global
platform CMS template routes.

Fixture (QA data only, reverted): MEMBER in E2E_COMPANY_ID gets direct grants platform.cms.view +
disclosure_type.manage (qa_rw). Calls use a template id that does not exist, so a request that
passes the permission gate ends in 404/400 and changes nothing:
- old binary: the gate passes -> 404/400;
- fixed binary: 403 at the gate (read, write = workflow publish, config write).
Control: the CMS persona (holds cms.template.*) still passes the gates (404, not 403).
"""
import json
import sys
import urllib.error
import urllib.request
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[4] / "scripts" / "devqa"))
from devqa_env import base_url, optional, persona, run_sql  # noqa: E402

BASE = base_url()
OWN = optional("E2E_COMPANY_ID", "c_001")
GRANTS = ("platform.cms.view", "disclosure_type.manage")
NOPE = "qa-smoke-role08-no-such-template"
ROUTES = {
    "read (GET global workflow)": ("GET", f"/api/v1/platform/cms/templates/{NOPE}/workflow", None),
    "write (POST workflow publish)": ("POST", f"/api/v1/platform/cms/templates/{NOPE}/workflow/publish", {"note": "qa smoke"}),
    "config write (PUT deadline config)": ("PUT", f"/api/v1/admin/disclosure-types/{NOPE}/config", {"deadline_config": {}}),
}
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
        return (json.loads(raw).get("error") or {}).get("code", "")
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


def perms(token):
    st, raw = call("GET", "/api/v1/me/effective-access", token)
    return (bodyj(raw).get("permissions") or []) if st == 200 else []


def report():
    print()
    for ok, name, detail in results:
        print(f"{'PASS' if ok else 'FAIL'}  {name}  {detail}")
    print(f"\n{sum(r[0] for r in results)}/{len(results)} passed")


def main():
    ent, _ = login("ENT", OWN)
    cms, _ = login("CMS", OWN)
    mem, m_mem = login("MEMBER", OWN)
    if not (m_mem or "").startswith("m_qa_"):
        raise SystemExit(f"refusing to run: MEMBER is not a QA membership: {m_mem}")
    normal = sorted(perms(mem))
    for name, (method, path, body) in ROUTES.items():
        st, raw = call(method, path, cms, body)
        record(st != 403, f"control: CMS operator passes the {name} gate", f"{st} {code_of(raw)}")
    try:
        for code in GRANTS:
            run_sql("INSERT INTO membership_direct_permissions (membership_id, company_id, permission_code, granted_by) "
                    f"VALUES ({q(m_mem)}, {q(OWN)}, {q(code)}, 'qa_smoke_role08') "
                    "ON DUPLICATE KEY UPDATE revoked_at=NULL, revoked_by=NULL, granted_by='qa_smoke_role08'", write=True)
        call("PATCH", f"/api/v1/admin/memberships/{m_mem}", ent, {"status": "active"})  # drops the cached access (H17)
        now = perms(mem)
        record(all(c in now for c in GRANTS) and "cms.template.write" not in now,
               "fixture: MEMBER holds platform.cms.view + disclosure_type.manage only", [c for c in GRANTS if c in now])
        for name, (method, path, body) in ROUTES.items():
            st, raw = call(method, path, mem, body)
            record(st == 403, f"ROLE-08 tenant alias refused at the {name} gate", f"{st} {code_of(raw)}")
    finally:
        run_sql("UPDATE membership_direct_permissions SET revoked_at=CURRENT_TIMESTAMP, revoked_by='qa_smoke_role08' "
                f"WHERE membership_id={q(m_mem)} AND revoked_at IS NULL AND granted_by='qa_smoke_role08'", write=True)
        call("PATCH", f"/api/v1/admin/memberships/{m_mem}", ent, {"status": "active"})
        left = run_sql(f"SELECT COUNT(*) FROM membership_direct_permissions WHERE membership_id={q(m_mem)} AND revoked_at IS NULL "
                       "AND granted_by='qa_smoke_role08'")
        record(left == ["0"], "cleanup: fixture grants revoked", left)
        mem2, _ = login("MEMBER", OWN)
        record(sorted(perms(mem2)) == normal, "cleanup: MEMBER permissions back to normal", len(perms(mem2)))


if __name__ == "__main__":
    try:
        main()
    finally:
        report()
    sys.exit(0 if results and all(r[0] for r in results) else 1)
