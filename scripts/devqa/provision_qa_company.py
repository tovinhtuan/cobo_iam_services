"""Provision a DEV company used only by QA smokes, with QA personas as its members.

Smokes that change ownership, the primary admin or company configuration must not touch a real
company. This script creates (once) "QA Persona Company (smoke only)" through the self-service
API as the ENT persona (POST /api/v1/company/create), so ENT is its owner and primary admin; then
adds ENT2 as a second admin and MEMBER as a plain member. Idempotent; the company id is stored
as E2E_QA_COMPANY_ID in ~/.cobo/dev-qa.env.

    python3 scripts/devqa/provision_qa_company.py            # create/sync
    python3 scripts/devqa/provision_qa_company.py --dry-run  # show the plan only
    python3 scripts/devqa/provision_qa_company.py --no-primary  # sync, then leave NO primary admin
                                                                # (last-admin smokes); rerun plain to restore

The writes bypass the API, so the QA company's effective-access cache generation is dropped after
every sync.

Run provision_qa_accounts.py first (personas must exist). Nothing secret is printed.
"""
import argparse
import json
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import devqa_env as env  # noqa: E402
from provision_qa_accounts import _run_root, update_env_file  # noqa: E402

COMPANY_NAME = "QA Persona Company (smoke only)"
# persona -> role_code in the QA company. ENT owns it (created by self-service, primary admin).
MEMBERS = {"ENT2": "admin_doanh_nghiep", "MEMBER": "user_thuong"}


def membership_id(p):
    return f"m_qa_{p.lower()}_qaco"


def api(method, path, token=None, body=None, headers=None):
    req = urllib.request.Request(env.base_url() + path, method=method,
                                 data=json.dumps(body).encode() if body is not None else None,
                                 headers={"Content-Type": "application/json", **(headers or {})})
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=25) as r:
            raw = json.loads(r.read() or b"{}")
            return r.status, raw.get("data", raw) if isinstance(raw, dict) else raw
    except urllib.error.HTTPError as e:
        return e.code, {}


def persona_token(p):
    email, password = env.persona(p)
    status, body = api("POST", "/api/v1/auth/login", body={"email": email, "password": password})
    if status != 200:
        raise SystemExit(f"{p} persona login failed: HTTP {status}")
    sess = body.get("session", {})
    if sess.get("access_token"):
        return sess["access_token"]
    status, sel = api("POST", "/api/v1/auth/select-company", sess.get("pre_company_token"),
                      {"company_id": env.optional("E2E_COMPANY_ID", "c_001")})
    if status != 200:
        raise SystemExit(f"{p} persona select-company failed: HTTP {status}")
    return sel["access_token"]


def q(v):
    return "'" + str(v).replace("'", "''") + "'"


def company_exists(company_id):
    return bool(company_id) and _run_root(f"SELECT COUNT(*) FROM companies WHERE company_id={q(company_id)};").strip() == "1"


def drop_access_cache(company_id):
    """A missing generation key means nothing cached for the company is used (it is reseeded)."""
    key = f"cobo_iam:effective_access_gen:v2:{company_id}"
    remote = f"docker exec {env.optional('DEV_REDIS_CONTAINER', 'cobo-iam-redis')} redis-cli DEL {key}"
    r = subprocess.run(env.ssh_cmd() + [remote], capture_output=True, timeout=60)
    print(f"effective-access cache generation dropped: {r.returncode == 0}")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dry-run", action="store_true")
    ap.add_argument("--no-primary", action="store_true", help="leave the QA company without a primary admin")
    args = ap.parse_args()
    env.load()

    company_id = env.optional("E2E_QA_COMPANY_ID")
    if company_exists(company_id):
        print(f"QA company exists: {company_id}")
    else:
        found = _run_root(f"SELECT company_id FROM companies WHERE company_name={q(COMPANY_NAME)} LIMIT 1;").strip()
        if found:
            company_id = found
            print(f"QA company found by name: {company_id}")
        elif args.dry_run:
            print(f"dry-run: would create company {COMPANY_NAME!r} as ENT via POST /api/v1/company/create")
            return
        else:
            # Fixed key: a retried run replays the same creation instead of making a second company.
            status, body = api("POST", "/api/v1/company/create", persona_token("ENT"), {"company_name": COMPANY_NAME},
                               {"Idempotency-Key": "qa-persona-company-v1"})
            if status not in (200, 201) or not body.get("company_id"):
                raise SystemExit(f"create QA company failed: HTTP {status}")
            company_id = body["company_id"]
            print(f"QA company created: {company_id}")

    plan = ["ENT    owner / primary admin (self-service bootstrap)"]
    plan += [f"{p:6} {membership_id(p)} role={role}" for p, role in MEMBERS.items()]
    print("\n".join(plan))
    if args.dry_run:
        print("dry-run: nothing changed")
        return

    sql = ["START TRANSACTION;"]
    for p, role in MEMBERS.items():
        uid = f"u_qa_persona_{p.lower()}"
        mid = membership_id(p)
        sql += [
            "INSERT INTO memberships (membership_id, user_id, company_id, membership_status) "
            f"VALUES ({q(mid)}, {q(uid)}, {q(company_id)}, 'active') ON DUPLICATE KEY UPDATE membership_status='active';",
            "INSERT INTO membership_roles (membership_id, role_id, status) "
            f"SELECT {q(mid)}, r.role_id, 'active' FROM roles r WHERE r.company_id={q(company_id)} AND r.role_code={q(role)} "
            "ON DUPLICATE KEY UPDATE status='active';",
            # exactly this role: a smoke that changed the role (e.g. primary-role) is undone here
            f"DELETE mr FROM membership_roles mr JOIN roles r ON r.role_id=mr.role_id WHERE mr.membership_id={q(mid)} AND r.role_code<>{q(role)};",
        ]
    # ENT keeps exactly the admin role and stays active. A smoke that deleted ENT's membership
    # (e.g. a last-admin guard missing on an old binary) gets it back under its recorded id.
    ent_mid = env.optional("E2E_QA_ENT_MEMBERSHIP_ID", "m_qa_ent_qaco")
    sql += [
        "INSERT INTO memberships (membership_id, user_id, company_id, membership_status) "
        f"SELECT {q(ent_mid)}, 'u_qa_persona_ent', {q(company_id)}, 'active' FROM DUAL WHERE NOT EXISTS "
        f"(SELECT 1 FROM memberships WHERE user_id='u_qa_persona_ent' AND company_id={q(company_id)});",
        "UPDATE memberships SET membership_status='active' WHERE user_id='u_qa_persona_ent' "
        f"AND company_id={q(company_id)};",
        "INSERT INTO membership_roles (membership_id, role_id, status) SELECT m.membership_id, r.role_id, 'active' "
        f"FROM memberships m JOIN roles r ON r.company_id=m.company_id AND r.role_code='admin_doanh_nghiep' "
        f"WHERE m.user_id='u_qa_persona_ent' AND m.company_id={q(company_id)} ON DUPLICATE KEY UPDATE status='active';",
        "DELETE mr FROM membership_roles mr JOIN memberships m ON m.membership_id=mr.membership_id "
        "JOIN roles r ON r.role_id=mr.role_id WHERE m.user_id='u_qa_persona_ent' "
        f"AND m.company_id={q(company_id)} AND r.role_code<>'admin_doanh_nghiep';",
    ]
    # Exactly one primary admin: ENT (restores the fixture after an ownership smoke), or none.
    primary = "FALSE" if args.no_primary else "(user_id = 'u_qa_persona_ent')"
    sql += [
        f"UPDATE memberships SET is_primary_admin = {primary} WHERE company_id={q(company_id)};",
        "COMMIT;",
        "SELECT m.membership_id, m.membership_status, m.is_primary_admin, GROUP_CONCAT(r.role_code) FROM memberships m "
        "LEFT JOIN membership_roles mr ON mr.membership_id=m.membership_id AND mr.status='active' "
        f"LEFT JOIN roles r ON r.role_id=mr.role_id WHERE m.company_id={q(company_id)} GROUP BY m.membership_id ORDER BY m.membership_id;",
    ]
    print(_run_root("\n".join(sql)).strip())
    drop_access_cache(company_id)
    ent_mid = _run_root(f"SELECT membership_id FROM memberships WHERE user_id='u_qa_persona_ent' AND company_id={q(company_id)};").strip()
    update_env_file({"E2E_QA_COMPANY_ID": company_id, "E2E_QA_ENT_MEMBERSHIP_ID": ent_mid})
    print(f"env file updated: E2E_QA_COMPANY_ID, E2E_QA_ENT_MEMBERSHIP_ID ({env.env_path()})")


if __name__ == "__main__":
    main()
