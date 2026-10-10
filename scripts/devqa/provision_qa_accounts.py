"""Provision DEV QA persona accounts and the qa_ro / qa_rw MySQL users.

Idempotent: safe to re-run. Passwords already in ~/.cobo/dev-qa.env are reused (the server is
re-synced to them); missing ones are generated locally and written straight into that file.
Nothing secret is printed. Convention: ../cobo_web_design/docs/ai-cache/dev-qa/README.md

    python3 scripts/devqa/provision_qa_accounts.py            # create/sync, then verify logins
    python3 scripts/devqa/provision_qa_accounts.py --rotate   # new passwords for everything
    python3 scripts/devqa/provision_qa_accounts.py --dry-run  # show the plan, change nothing

Requires: DEV_SSH_* in the env file (key auth), Go toolchain (bcrypt helper), run from any cwd.
Targets DEV only - it writes to the server database over SSH as the container's root user.
"""
import argparse
import json
import re
import secrets
import subprocess
import sys
import urllib.error
import urllib.request
import uuid
from pathlib import Path

HERE = Path(__file__).resolve().parent
IAM_ROOT = HERE.parents[1]
sys.path.insert(0, str(HERE))
import devqa_env as env  # noqa: E402

ROLE = {
    ("c_001", "admin_doanh_nghiep"): "r0000001-0001-4000-8000-000000000012",
    ("c_001", "user_thuong"): "r0000001-0001-4000-8000-000000000015",
    ("c_001", "cms_operator"): "r0000001-0001-4000-8000-000000000016",
    ("c_002", "admin_doanh_nghiep"): "r0000001-0001-4000-8000-000000000018",
}

# persona -> (full name, [(company, role_code)])
PERSONAS = {
    "ENT": ("QA Persona ENT (tenant admin)", [("c_001", "admin_doanh_nghiep")]),
    "ENT2": ("QA Persona ENT2 (approver)", [("c_001", "admin_doanh_nghiep")]),
    "MEMBER": ("QA Persona MEMBER", [("c_001", "user_thuong")]),
    "CMS": ("QA Persona CMS (platform operator)", [("c_001", "cms_operator")]),
    "USER": ("QA Persona USER (multi-company)", [("c_001", "admin_doanh_nghiep"), ("c_002", "admin_doanh_nghiep")]),
}
EMAIL_FMT = "qa.persona.{p}@cobo.test"
COMPANY_ID, OTHER_COMPANY_ID = "c_001", "c_002"

# DB users: local socket only (docker exec), never reachable through the published 3306 port.
DB_USERS = {
    "QA_DB": ("qa_ro", ["GRANT SELECT ON cobo_iam.* TO 'qa_ro'@'localhost'"]),
    "QA_DB_RW": ("qa_rw", [
        "GRANT SELECT ON cobo_iam.* TO 'qa_rw'@'localhost'",
        # fixture tables written by smokes (role01-approval*); extend deliberately when needed
        "GRANT INSERT, UPDATE, DELETE ON cobo_iam.role_permissions TO 'qa_rw'@'localhost'",
        "GRANT INSERT, UPDATE, DELETE ON cobo_iam.membership_direct_permissions TO 'qa_rw'@'localhost'",
    ]),
}

NS = uuid.UUID("6f0c2a52-7d1e-4f39-9a5e-0c0ba0de9a11")


def new_password():
    return secrets.token_urlsafe(24)  # [A-Za-z0-9_-] only: safe inside SQL quotes


def bcrypt_hashes(passwords):
    r = subprocess.run(
        ["go", "run", "./scripts/devqa/bcrypthash"], cwd=IAM_ROOT, input="\n".join(passwords) + "\n",
        capture_output=True, text=True, timeout=300,
    )
    if r.returncode != 0:
        raise SystemExit(f"bcrypt helper failed: {r.stderr[:300]}")
    hashes = r.stdout.split()
    if len(hashes) != len(passwords) or not all(h.startswith("$2") for h in hashes):
        raise SystemExit("bcrypt helper returned unexpected output")
    return hashes


def update_env_file(values):
    """Sets keys in the local env file in place (keeps comments/order, appends missing keys)."""
    path = env.env_path()
    path.parent.mkdir(parents=True, exist_ok=True)
    lines = path.read_text(encoding="utf-8").splitlines() if path.exists() else []
    pending = dict(values)
    for i, line in enumerate(lines):
        m = re.match(r"^(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=", line.strip())
        if m and m.group(1) in pending:
            lines[i] = f"{m.group(1)}={pending.pop(m.group(1))}"
    if pending:
        lines += ["", "# --- added by provision_qa_accounts.py"] + [f"{k}={v}" for k, v in pending.items()]
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def persona_sql(p, password_hash):
    name, grants = PERSONAS[p]
    email = EMAIL_FMT.format(p=p.lower())
    uid = f"u_qa_persona_{p.lower()}"
    cred = str(uuid.uuid5(NS, f"cred:{uid}"))
    out = [
        "INSERT INTO users (user_id, login_id, full_name, email, email_verified_at, account_status, locked_until) "
        f"VALUES ('{uid}', '{email}', '{name}', '{email}', CURRENT_TIMESTAMP(3), 'active', NULL) "
        "ON DUPLICATE KEY UPDATE full_name=VALUES(full_name), email=VALUES(email), "
        "email_verified_at=COALESCE(email_verified_at, VALUES(email_verified_at)), account_status='active', locked_until=NULL;",
        "INSERT INTO user_subscription_tiers (user_id, subscription_tier, source) "
        f"VALUES ('{uid}', 'Enterprise', 'qa_persona') ON DUPLICATE KEY UPDATE subscription_tier='Enterprise', source='qa_persona';",
        "INSERT INTO credentials (credential_id, user_id, credential_type, password_hash, password_algo, password_changed_at, status) "
        f"VALUES ('{cred}', '{uid}', 'password', '{password_hash}', 'bcrypt', CURRENT_TIMESTAMP(3), 'active') "
        "ON DUPLICATE KEY UPDATE password_hash=VALUES(password_hash), password_algo='bcrypt', "
        "password_changed_at=VALUES(password_changed_at), status='active';",
    ]
    for company, role_code in grants:
        mid = f"m_qa_{p.lower()}_{company.replace('_', '')}"
        out += [
            "INSERT INTO memberships (membership_id, user_id, company_id, membership_status) "
            f"VALUES ('{mid}', '{uid}', '{company}', 'active') ON DUPLICATE KEY UPDATE membership_status='active';",
            "INSERT INTO membership_roles (membership_id, role_id, status) "
            f"SELECT m.membership_id, '{ROLE[(company, role_code)]}', 'active' FROM memberships m "
            f"WHERE m.user_id='{uid}' AND m.company_id='{company}' ON DUPLICATE KEY UPDATE status='active';",
        ]
    return out


def db_user_sql(prefix, password):
    user, grants = DB_USERS[prefix]
    return [
        f"CREATE USER IF NOT EXISTS '{user}'@'localhost' IDENTIFIED BY '{password}';",
        f"ALTER USER '{user}'@'localhost' IDENTIFIED BY '{password}';",
        f"REVOKE ALL PRIVILEGES, GRANT OPTION FROM '{user}'@'localhost';",
    ] + [g + ";" for g in grants]


def check_login(base, email, password, company):
    req = urllib.request.Request(
        base + "/api/v1/auth/login", method="POST",
        data=json.dumps({"email": email, "password": password}).encode(),
        headers={"Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=25) as r:
            body = json.loads(r.read())
    except urllib.error.HTTPError as e:
        return f"HTTP {e.code}"
    body = body.get("data", body)
    # single-company users are auto-selected (current_context); multi-company users get memberships[]
    companies = {m.get("company_id") for m in body.get("memberships", [])}
    companies.add((body.get("current_context") or {}).get("company_id"))
    companies = sorted(c for c in companies if c)
    return f"200 companies={companies}" + ("" if company in companies else " (missing expected company!)")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--rotate", action="store_true", help="generate new passwords for every persona and DB user")
    ap.add_argument("--dry-run", action="store_true", help="print the plan only")
    args = ap.parse_args()

    env.load()
    env.require("DEV_SSH_HOST")
    file_updates = {}
    if not env.optional("E2E_BASE_URL"):
        file_updates["E2E_BASE_URL"] = f"http://{env.require('DEV_SSH_HOST')}:3000"
    for key, val in (("E2E_COMPANY_ID", COMPANY_ID), ("E2E_OTHER_COMPANY_ID", OTHER_COMPANY_ID)):
        if not env.optional(key):
            file_updates[key] = val

    passwords, plan = {}, []
    for p in PERSONAS:
        file_updates[f"E2E_{p}_EMAIL"] = EMAIL_FMT.format(p=p.lower())
        existing = "" if args.rotate else env.optional(f"E2E_{p}_PASSWORD")
        passwords[p] = existing or new_password()
        plan.append(f"persona {p:6} {EMAIL_FMT.format(p=p.lower()):30} {PERSONAS[p][1]} password={'reuse' if existing else 'new'}")
        if not existing:
            file_updates[f"E2E_{p}_PASSWORD"] = passwords[p]
    db_passwords = {}
    for prefix, (user, _) in DB_USERS.items():
        existing = "" if args.rotate else env.optional(f"{prefix}_PASSWORD")
        db_passwords[prefix] = existing or new_password()
        file_updates[f"{prefix}_USER"] = user
        plan.append(f"db user {user}@localhost password={'reuse' if existing else 'new'}")
        if not existing:
            file_updates[f"{prefix}_PASSWORD"] = db_passwords[prefix]

    print("Target DB via SSH:", env.require("DEV_SSH_HOST"))
    print("\n".join(plan))
    if args.dry_run:
        print("dry-run: nothing changed")
        return

    hashes = dict(zip(PERSONAS, bcrypt_hashes([passwords[p] for p in PERSONAS])))
    app_sql = ["START TRANSACTION;"]
    for p in PERSONAS:
        app_sql += persona_sql(p, hashes[p])
    app_sql.append("COMMIT;")
    # Root is required for CREATE USER / GRANT; app rows use it too (qa_rw is deliberately narrow).
    _run_root("\n".join(app_sql))
    db_sql = []
    for prefix in DB_USERS:
        db_sql += db_user_sql(prefix, db_passwords[prefix])
    _run_root("\n".join(db_sql))

    # Persist only after the server accepted the change, so file and server stay in sync.
    update_env_file(file_updates)
    print(f"env file updated: {env.env_path()} ({len(file_updates)} keys; values not shown)")

    base = (env.optional("E2E_BASE_URL") or file_updates.get("E2E_BASE_URL", "")).rstrip("/")
    ok = True
    for p in PERSONAS:
        res = check_login(base, EMAIL_FMT.format(p=p.lower()), passwords[p], PERSONAS[p][1][0][0])
        ok &= res.startswith("200") and "missing" not in res
        print(f"login {p:6} -> {res}")
    for prefix, (user, _) in DB_USERS.items():
        rows = _run_as(user, db_passwords[prefix], "SELECT CURRENT_USER(), COUNT(*) FROM users WHERE login_id LIKE 'qa.persona.%';")
        print(f"db {user:6} -> {rows}")
    if not ok:
        raise SystemExit("some persona logins failed")


def _run_root(sql):
    remote = (
        f"docker exec -i {env.optional('DEV_MYSQL_CONTAINER', 'cobo-iam-mysql')} "
        "sh -c 'mysql -uroot -p\"$MYSQL_ROOT_PASSWORD\" cobo_iam -N'"
    )
    r = subprocess.run(env.ssh_cmd() + [remote], input=sql.encode("utf-8"), capture_output=True, timeout=120)
    if r.returncode != 0:
        raise SystemExit(f"sql failed (exit {r.returncode}): {r.stderr.decode('utf-8', 'replace')[:300]}")
    return r.stdout.decode("utf-8", "replace")


def _run_as(user, password, sql):
    remote = (
        "IFS= read -r MYSQL_PWD; export MYSQL_PWD; "
        f"exec docker exec -i -e MYSQL_PWD {env.optional('DEV_MYSQL_CONTAINER', 'cobo-iam-mysql')} mysql -u{user} cobo_iam -N"
    )
    # bytes, not text: Windows text mode would append a CR to the password line
    payload = (password + "\n" + sql).encode("utf-8")
    r = subprocess.run(env.ssh_cmd() + [remote], input=payload, capture_output=True, timeout=60)
    out, err = r.stdout.decode("utf-8", "replace").strip(), r.stderr.decode("utf-8", "replace").strip()
    return out if r.returncode == 0 else f"FAILED: {err[:200]}"


if __name__ == "__main__":
    main()
