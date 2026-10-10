"""DEV smoke for ROLE-24: an admin.* action without its own policy is denied, not gated by
system.settings.

QA company only (E2E_QA_COMPANY_ID). DEV has no action_policy_matrix table, so legacyPolicy decides.
Fixture (reverted): MEMBER gets a direct system.settings grant (qa_rw). Through the token-bound
POST /internal/v1/authorize (+ /batch) on the API origin (not proxied by the portal):
- MEMBER, admin.qa_smoke.unknown -> deny (old binary: allow via system.settings);
- MEMBER, admin.roles.list (mapped to rbac.manage) -> deny; ENT -> allow (mapped actions unchanged);
- a body subject naming another company is ignored (decision stays in the token's company).
"""
import json
import sys
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[4] / "scripts" / "devqa"))
from devqa_env import base_url, optional, persona, require, run_sql  # noqa: E402

BASE = base_url()
# The portal nginx does not proxy /internal/*; those routes live on the API origin.
API = optional("E2E_API_BASE_URL", urllib.parse.urlsplit(BASE)._replace(netloc=urllib.parse.urlsplit(BASE).hostname + ":8080").geturl()).rstrip("/")
QA = require("E2E_QA_COMPANY_ID")
OTHER = optional("E2E_OTHER_COMPANY_ID", "c_002")
A = "/api/v1/admin"
UNKNOWN = "admin.qa_smoke.unknown"
results = []


def call(method, path, token=None, body=None):
    origin = API if path.startswith("/internal/") else BASE
    req = urllib.request.Request(origin + path, method=method, data=json.dumps(body).encode() if body is not None else None,
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
    return run_sql(f"SELECT COUNT(*) FROM membership_direct_permissions WHERE membership_id={q(mid)} "
                   "AND permission_code='system.settings' AND revoked_at IS NULL AND granted_by='qa_smoke_role24'")


def decide(token, action, subject=None):
    st, raw = call("POST", "/internal/v1/authorize", token, {"subject": subject or {}, "action": action,
                                                              "resource": {"type": "admin_access", "id": QA}})
    d = bodyj(raw)
    return st, d.get("decision"), d.get("matched_permissions"), code_of(raw)


def report():
    print()
    for ok, name, detail in results:
        print(f"{'PASS' if ok else 'FAIL'}  {name}  {detail}")
    print(f"\n{sum(r[0] for r in results)}/{len(results)} passed")


def main():
    ent, m_ent = login("ENT", QA)
    member, m_mem = login("MEMBER", QA)
    try:
        run_sql("INSERT INTO membership_direct_permissions (membership_id, company_id, permission_code, granted_by) "
                f"VALUES ({q(m_mem)}, {q(QA)}, 'system.settings', 'qa_smoke_role24') "
                "ON DUPLICATE KEY UPDATE revoked_at=NULL, revoked_by=NULL, granted_by='qa_smoke_role24'", write=True)
        call("PATCH", f"{A}/memberships/{m_mem}", ent, {"status": "active"})  # drop cached access
        record(grant_active(m_mem) == ["1"], "fixture: MEMBER holds system.settings directly", "")

        st, dec, matched, err = decide(member, UNKNOWN)
        record(st == 200 and dec == "deny", "ROLE-24 unknown admin action denied for a system.settings holder", f"{st} {dec} {matched} {err}")
        st, raw = call("POST", "/internal/v1/authorize/batch", member, {"subject": {}, "checks": [
            {"action": UNKNOWN, "resource": {"type": "admin_access", "id": QA}}]})
        res = (bodyj(raw).get("results") or [{}])[0]
        record(st == 200 and res.get("decision") == "deny", "ROLE-24 same via /batch", f"{st} {res.get('decision')} {code_of(raw)}")
        st, dec, matched, err = decide(member, UNKNOWN, {"membership_id": m_ent, "company_id": OTHER})
        record(st == 200 and dec == "deny", "body subject (other company / ENT) ignored", f"{st} {dec} {err}")

        st, dec, matched, err = decide(member, "admin.roles.list")
        record(st == 200 and dec == "deny", "mapped admin action: MEMBER without rbac.manage denied", f"{st} {dec} {err}")
        st, dec, matched, err = decide(ent, "admin.roles.list")
        record(st == 200 and dec == "allow" and matched == ["rbac.manage"], "mapped admin action: ENT allowed via rbac.manage", f"{st} {dec} {matched} {err}")
    finally:
        run_sql("UPDATE membership_direct_permissions SET revoked_at=CURRENT_TIMESTAMP, revoked_by='qa_smoke_role24' "
                f"WHERE membership_id={q(m_mem)} AND revoked_at IS NULL AND granted_by='qa_smoke_role24'", write=True)
        call("PATCH", f"{A}/memberships/{m_mem}", ent, {"status": "active"})
        record(grant_active(m_mem) == ["0"], "cleanup: fixture grant revoked", grant_active(m_mem))
        st, dec, _, _ = decide(member, "company.view")
        record(st == 200, "cleanup: MEMBER token still valid after cleanup", f"{st} {dec}")


if __name__ == "__main__":
    try:
        main()
    finally:
        report()
    sys.exit(0 if results and all(r[0] for r in results) else 1)
