"""DEV smoke for RP-10 / H16: transferring ownership is atomic (exactly one primary admin).

Runs only inside the QA company (E2E_QA_COMPANY_ID, created by scripts/devqa/provision_qa_company.py)
with QA personas: ENT (owner), ENT2 (admin), MEMBER. Ownership moves ENT -> ENT2 -> back to ENT;
the finally block restores ENT as the only primary admin (falls back to the provisioning script).
Credentials: ~/.cobo/dev-qa.env via scripts/devqa/devqa_env.py; never printed.
"""
import json
import subprocess
import sys
import threading
import urllib.error
import urllib.request
from pathlib import Path

DEVQA = Path(__file__).resolve().parents[4] / "scripts" / "devqa"
sys.path.insert(0, str(DEVQA))
from devqa_env import base_url, optional, persona, require, run_sql  # noqa: E402

BASE = base_url()
QA = require("E2E_QA_COMPANY_ID")
OWN = optional("E2E_COMPANY_ID", "c_001")
A = "/api/v1/admin"
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
        raise SystemExit(f"login failed for persona {p}: HTTP {st} {code_of(raw)}")
    body = bodyj(raw)
    sess = body.get("session", {})
    mids = {m["company_id"]: m["membership_id"] for m in body.get("memberships", [])}
    ctx = body.get("current_context") or {}
    if ctx.get("company_id"):
        mids.setdefault(ctx["company_id"], ctx.get("membership_id"))
    if sess.get("access_token") and ctx.get("company_id") == company:
        return sess["access_token"], mids[company]
    st, raw = call("POST", "/api/v1/auth/select-company", sess.get("pre_company_token") or sess.get("access_token"), {"company_id": company})
    if st != 200:
        raise SystemExit(f"select-company {company} failed for persona {p}: HTTP {st} {code_of(raw)}")
    return bodyj(raw)["access_token"], mids[company]


def q(v):
    return "'" + str(v).replace("'", "''") + "'"


def primaries():
    return run_sql(f"SELECT membership_id FROM memberships WHERE company_id={q(QA)} AND is_primary_admin=1 ORDER BY membership_id")


def transfer(token, target):
    return call("POST", f"{A}/company/transfer-ownership", token, {"target_membership_id": target})


def report():
    print()
    for ok, name, detail in results:
        print(f"{'PASS' if ok else 'FAIL'}  {name}  {detail}")
    print(f"\n{sum(r[0] for r in results)}/{len(results)} passed")


def main():
    ent, m_ent = login("ENT", QA)
    ent2, m_ent2 = login("ENT2", QA)
    mem, m_mem = login("MEMBER", QA)
    _, m_ent2_own = login("ENT2", OWN)
    print(f"qa company={QA} ent={m_ent} ent2={m_ent2} member={m_mem}")
    record(primaries() == [m_ent], "fixture: ENT is the only primary admin", primaries())

    try:
        # --- guards ------------------------------------------------------------------------------
        st, raw = transfer(mem, m_ent2)
        record(st == 403, "MEMBER cannot transfer ownership", f"{st} {code_of(raw)}")
        st, raw = transfer(ent, m_ent)
        record(st == 400, "transfer to self rejected", f"{st} {code_of(raw)}")
        st, raw = transfer(ent, m_ent2_own)
        record(st in (403, 404), "transfer to a membership of another company rejected", f"{st} {code_of(raw)}")
        st, raw = transfer(ent, m_mem)
        record(st == 409 and code_of(raw) == "STATE_CONFLICT", "ROLE-26 transfer to a non-admin member rejected", f"{st} {code_of(raw)}")
        st, raw = call("PATCH", f"{A}/memberships/{m_mem}", ent, {"status": "inactive"})
        record(st == 200, "fixture: deactivate MEMBER", st)
        st, raw = transfer(ent, m_mem)
        record(st == 409 and code_of(raw) == "STATE_CONFLICT", "transfer to an inactive member rejected", f"{st} {code_of(raw)}")
        audit = run_sql("SELECT COUNT(*) FROM audit_logs WHERE action='admin.company.ownership.transfer' "
                        f"AND company_id={q(QA)} AND created_at > NOW() - INTERVAL 10 MINUTE")
        before_audit = int(audit[0]) if audit else 0
        st, _ = call("PATCH", f"{A}/memberships/{m_mem}", ent, {"status": "active"})
        record(st == 200, "fixture: reactivate MEMBER", st)
        record(primaries() == [m_ent], "owner unchanged after rejected transfers", primaries())

        # --- happy path ----------------------------------------------------------------------------
        st, raw = transfer(ent, m_ent2)
        record(st == 200, "ENT transfers ownership to ENT2", f"{st} {code_of(raw)}")
        record(primaries() == [m_ent2], "DB: ENT2 is the only primary admin", primaries())
        meta = run_sql("SELECT JSON_UNQUOTE(JSON_EXTRACT(metadata_json,'$.from_membership_id')) FROM audit_logs "
                       f"WHERE action='admin.company.ownership.transfer' AND company_id={q(QA)} ORDER BY created_at DESC LIMIT 1")
        record(meta == [m_ent], "ROLE-28 audit records the previous owner", meta)
        st, raw = transfer(ent, m_mem)
        record(st == 403, "former owner can no longer transfer", f"{st} {code_of(raw)}")

        # --- concurrency: two transfers by the owner at the same time ---------------------------
        outcomes = {}
        start = threading.Barrier(2)

        def go(target):
            start.wait()
            outcomes[threading.current_thread().name] = transfer(ent2, target)

        threads = [threading.Thread(target=go, args=(t,)) for t in (m_ent, m_ent)]
        for th in threads:
            th.start()
        for th in threads:
            th.join()
        statuses = sorted(o[0] for o in outcomes.values())
        record(statuses.count(200) == 1 and all(s in (200, 403, 409) for s in statuses),
               "concurrent transfers: exactly one succeeds", {k: (v[0], code_of(v[1])) for k, v in outcomes.items()})
        p = primaries()
        record(len(p) == 1, "DB: exactly one primary admin after concurrent transfers", p)
    finally:
        # --- restore: ENT owns the QA company again ---------------------------------------------
        p = primaries()
        if p != [m_ent] and len(p) == 1:
            owner_persona = {m_ent2: "ENT2", m_mem: "MEMBER"}.get(p[0])
            if owner_persona:
                tok, _ = login(owner_persona, QA)
                transfer(tok, m_ent)
        if primaries() != [m_ent]:
            subprocess.run([sys.executable, "-I", str(DEVQA / "provision_qa_company.py")], capture_output=True, timeout=300)
        statuses = run_sql(f"SELECT membership_id, membership_status FROM memberships WHERE company_id={q(QA)} ORDER BY membership_id")
        record(primaries() == [m_ent], "cleanup: ENT is the only primary admin again", primaries())
        record(all(r.endswith("\tactive") for r in statuses), "cleanup: QA company memberships active", statuses)


if __name__ == "__main__":
    try:
        main()
    finally:
        report()
    sys.exit(0 if results and all(r[0] for r in results) else 1)
