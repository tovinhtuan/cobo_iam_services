#!/usr/bin/env python3
"""Sets users.email of DEV seed accounts (bug-login-old-accounts-2026-10-16, item 3).

The seed accounts rotated by OPS-1 have no mailbox (example.com or none), so "forgot password" cannot
reach anyone. Give each one its own address (e.g. a Gmail "+alias" of the person using it). Every
address must be unused: no other account may have it as email or login_id (see BUG-2).

Run from cobo_iam_services; without --apply it only prints the plan:
  python3 docs/ai-cache/bug-login-old-accounts-2026-10-16/refs/set_seed_emails_dev.py \
      u_admin_dn=<address> u_cms_operator=<address> [--apply]

Reads use qa_ro. qa_rw cannot update `users`, so --apply writes as the MySQL container's root over
SSH (same path as OPS-1 / provision_qa_accounts.py; the root password never leaves the container).
Old values are saved to ~/.cobo/backups/ (mode 600) first. Addresses are masked in the output.
"""
import datetime
import json
import os
import re
import subprocess
import sys

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "../../../../scripts/devqa"))
import devqa_env as env  # noqa: E402

# The 15 seed accounts rotated by OPS-1; nothing else may be changed by this script.
SEED_IDS = {
    "u_admin_dn", "u_admin_web", "u_cms_operator", "u_platform_tenant_admin", "u_123", "u_single",
    "u_no_company", "u_truong_phong", "u_truong_nhom", "u_nhan_vien", "usr_org_admin_001",
    "usr_org_nv_nhom_001", "usr_org_nv_phong_001", "usr_org_tn_001", "usr_org_tp_001",
}
ADDRESS = re.compile(r"^[a-z0-9._%+-]+@[a-z0-9-]+(\.[a-z0-9-]+)*\.[a-z]{2,}$")


def mask(addr):
    if not addr:
        return "(none)"
    local, _, domain = addr.partition("@")
    base, plus, tag = local.partition("+")
    return f"{base[:3]}…{plus}{tag}@{domain}"


def lookup(addr):
    """User ids the recovery lookup would consider for `addr` (email or login_id)."""
    rows = env.run_sql(
        f"SELECT user_id FROM users WHERE LOWER(TRIM(email)) = '{addr}' OR LOWER(TRIM(login_id)) = '{addr}' ORDER BY user_id"
    )
    return [r.strip() for r in rows]


def main(argv):
    apply = "--apply" in argv
    plan = {}
    for arg in (a for a in argv if a != "--apply"):
        uid, sep, addr = arg.partition("=")
        addr = addr.strip().lower()
        if not sep or uid not in SEED_IDS or not ADDRESS.match(addr):
            raise SystemExit(f"bad argument {arg.split('=')[0]}=…: want <seed user_id>=<email address>")
        plan[uid] = addr
    if not plan:
        raise SystemExit(__doc__)
    if len(set(plan.values())) != len(plan):
        raise SystemExit("each account needs its own address")

    ids = ",".join(f"'{u}'" for u in plan)
    old = {}
    for row in env.run_sql(f"SELECT user_id, IFNULL(email, '') FROM users WHERE user_id IN ({ids})"):
        uid, email = (row.split("\t") + [""])[:2]
        old[uid] = email
    missing = set(plan) - set(old)
    if missing:
        raise SystemExit(f"not found on DEV: {sorted(missing)}")
    for uid, addr in plan.items():
        others = [u for u in lookup(addr) if u != uid]
        if others:
            raise SystemExit(f"{uid}: {mask(addr)} already belongs to another account; pick another address")
        print(f"{uid}: {mask(old[uid])} -> {mask(addr)}")
    if not apply:
        print("dry run; add --apply to write")
        return

    stamp = datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    backup_dir = os.path.expanduser("~/.cobo/backups")
    os.makedirs(backup_dir, exist_ok=True)
    backup = os.path.join(backup_dir, f"bug-login-old-accounts-emails-{stamp}.json")
    fd = os.open(backup, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w") as f:
        json.dump({"taken_at": stamp, "old_email": old, "new_email": plan}, f, indent=2)
    print(f"backup: {backup}")

    stmts = ["START TRANSACTION;"]
    for uid, addr in plan.items():
        guard = "(email IS NULL OR email = '')" if not old[uid] else "email = '" + old[uid].replace("'", "''") + "'"
        stmts.append(f"UPDATE users SET email = '{addr}', updated_at = UTC_TIMESTAMP() WHERE user_id = '{uid}' AND {guard};")
    stmts.append("COMMIT;")
    container = env.optional("DEV_MYSQL_CONTAINER", "cobo-iam-mysql")
    remote = f"docker exec -i {container} sh -c 'mysql -uroot -p\"$MYSQL_ROOT_PASSWORD\" cobo_iam -N'"
    r = subprocess.run(env.ssh_cmd() + [remote], input="\n".join(stmts).encode(), capture_output=True, timeout=60)
    if r.returncode != 0:
        err = "\n".join(l for l in r.stderr.decode("utf-8", "replace").splitlines() if "Using a password" not in l)
        raise SystemExit(f"update failed (exit {r.returncode}): {err[:300]}")

    ok = True
    for uid, addr in plan.items():
        found = lookup(addr)
        good = found == [uid]
        ok &= good
        print(f"verify {uid}: {mask(addr)} -> {found} {'OK' if good else 'UNEXPECTED'}")
    raise SystemExit(0 if ok else 1)


if __name__ == "__main__":
    main(sys.argv[1:])
