"""Shared loader for DEV deploy/smoke credentials (Python side).

Credentials live OUTSIDE git, in one local file per machine:
    default: ~/.cobo/dev-qa.env      override: COBO_DEV_QA_ENV=/path/to/file
Template + convention: ../cobo_web_design/docs/ai-cache/dev-qa/ (README.md, dev-qa.env.example).
Node counterpart: ../cobo_web_design/scripts/lib/devQaEnv.mjs (same variable names).

Process env wins over the file. Never print or persist the values returned here.

Usage from a smoke script anywhere in the workspace:
    sys.path.insert(0, "<iam repo>/scripts/devqa")
    from devqa_env import base_url, persona, ssh_cmd, run_sql
"""
import os
import re
from pathlib import Path

PERSONAS = ("ENT", "ENT2", "MEMBER", "CMS", "USER")

_loaded_from = None


def env_path():
    return Path(os.environ.get("COBO_DEV_QA_ENV") or Path.home() / ".cobo" / "dev-qa.env")


def _parse(text):
    out = {}
    for raw in text.splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        m = re.match(r"^(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$", line)
        if not m:
            continue
        value = m.group(2).strip()
        if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
            value = value[1:-1]
        else:
            value = re.sub(r"\s+#.*$", "", value)
        out[m.group(1)] = value
    return out


def load():
    """Loads the local credential file into os.environ (existing env vars are kept)."""
    global _loaded_from
    if _loaded_from is not None:
        return _loaded_from
    path = env_path()
    if path.is_file():
        for k, v in _parse(path.read_text(encoding="utf-8")).items():
            if not os.environ.get(k):
                os.environ[k] = v
        _loaded_from = str(path)
    else:
        _loaded_from = ""
    return _loaded_from


def _fail(message):
    raise SystemExit(
        f"{message}\nSet it in {env_path()} (template: cobo_web_design/docs/ai-cache/dev-qa/dev-qa.env.example)"
        " or export it before running."
    )


def optional(name, default=""):
    load()
    return os.environ.get(name) or default


def require(name):
    value = optional(name)
    if not value:
        _fail(f"Missing required env {name}.")
    return value


def base_url():
    """Web origin of the target environment (nginx proxies /api). No trailing slash."""
    return require("E2E_BASE_URL").rstrip("/")


def persona(name):
    """(email, password) of a QA persona; E2E_<P>_PASSWORD falls back to E2E_PASSWORD.

    ENT: tenant admin with rbac.manage in E2E_COMPANY_ID. ENT2: second admin of the same company
    (approver for four-eyes flows). MEMBER: member without admin perms. CMS: platform.cms.view.
    USER: multi-company account used by older UI smokes.
    """
    key = name.upper()
    if key not in PERSONAS:
        _fail(f"Unknown persona {name}; expected one of {', '.join(PERSONAS)}.")
    email = require(f"E2E_{key}_EMAIL")
    password = optional(f"E2E_{key}_PASSWORD") or optional("E2E_PASSWORD")
    if not password:
        _fail(f"Missing E2E_{key}_PASSWORD (or shared E2E_PASSWORD).")
    return email, password


def ssh_cmd():
    """Argument list for a non-interactive SSH session to the DEV host (key auth only)."""
    host = require("DEV_SSH_HOST")
    cmd = ["ssh", "-o", "BatchMode=yes", "-p", optional("DEV_SSH_PORT", "22")]
    identity = optional("DEV_SSH_IDENTITY_FILE")
    if identity:
        cmd += ["-i", os.path.expanduser(identity)]
    return cmd + [f"{optional('DEV_SSH_USER', 'root')}@{host}"]


def run_sql(statement, database="cobo_iam", timeout=60, write=False):
    """Runs SQL on the DEV MySQL container over SSH and returns non-empty output lines.

    Reads use QA_DB_USER (read-only qa_ro); write=True uses QA_DB_RW_USER (smoke fixtures that
    must set up legacy/drift state). The password travels on SSH stdin and reaches mysql via
    MYSQL_PWD inside the remote shell, so it never appears on a command line. When the matching
    user is not configured, falls back to the container's root credentials (transition only).
    """
    import subprocess

    container = optional("DEV_MYSQL_CONTAINER", "cobo-iam-mysql")
    prefix = "QA_DB_RW" if write else "QA_DB"
    user = optional(f"{prefix}_USER")
    if user:
        remote = (
            "IFS= read -r MYSQL_PWD; export MYSQL_PWD; "
            f"exec docker exec -i -e MYSQL_PWD {container} mysql -u{user} {database} -N"
        )
        stdin = require(f"{prefix}_PASSWORD") + "\n" + statement
    else:
        remote = f"docker exec -i {container} sh -c 'mysql -uroot -p\"$MYSQL_ROOT_PASSWORD\" {database} -N'"
        stdin = statement
    # bytes, not text: Windows text mode turns LF into CRLF and the remote `read` would keep the CR
    r = subprocess.run(ssh_cmd() + [remote], input=stdin.encode("utf-8"), capture_output=True, timeout=timeout)
    if r.returncode != 0:
        raise SystemExit(f"sql failed (exit {r.returncode}): {r.stderr.decode('utf-8', 'replace')[:300]}")
    return [line for line in r.stdout.decode("utf-8", "replace").splitlines() if line.strip()]
