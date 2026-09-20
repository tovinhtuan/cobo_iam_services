#!/usr/bin/env python3
import subprocess
import sys

login = sys.argv[1] if len(sys.argv) > 1 else "qa.propose.only.2ce83604@example.com"
sql = (
    "UPDATE users SET email_verified_at=UTC_TIMESTAMP(), "
    "account_status='active', updated_at=UTC_TIMESTAMP() "
    f"WHERE login_id='{login}'; "
    "SELECT login_id, IF(email_verified_at IS NULL,0,1), account_status FROM users "
    f"WHERE login_id='{login}';"
)
cmd = [
    "docker", "exec", "cobo-iam-mysql",
    "mysql", "-uroot", "-proot", "cobo_iam", "-Nse", sql,
]
subprocess.run(cmd, check=False)
