#!/usr/bin/env python3
"""PreToolUse (Read|Edit|Write|MultiEdit|NotebookEdit): protect secrets and applied migrations."""
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from cobo_hooklib import (is_env_file, is_secret_key_file, pre_tool_decision,  # noqa: E402
                          project_dir, read_input, rel)

data = read_input()
tool = data.get("tool_name", "")
ti = data.get("tool_input", {}) or {}
path = ti.get("file_path") or ti.get("notebook_path") or ""
if not path:
    sys.exit(0)
base = project_dir(data)
abspath = path if os.path.isabs(path) else os.path.join(base, path)
r = rel(abspath, base)

if is_secret_key_file(abspath):
    pre_tool_decision("deny", f"{r} looks like a private key/keystore. Cobo rule: never read or edit secret material "
                              "with the agent. Ask the user to handle it manually (see secrets-cors-cookie-review).")

if is_env_file(abspath):
    if tool == "Read":
        pre_tool_decision("ask", f"{r} may contain real credentials. Reading it puts secrets into the transcript. "
                                 "Prefer the .example file or ask the user for the variable names only.")
    pre_tool_decision("deny", f"{r} holds local/server secrets and must not be edited by the agent. "
                              "Edit the matching *.example file and tell the user which variables to set.")

if tool in ("Edit", "Write", "MultiEdit"):
    norm = abspath.replace("\\", "/")
    if re.search(r"/migrations/\d{4}_[^/]+\.sql$", norm) and os.path.exists(abspath):
        pre_tool_decision("ask", f"{r} is an existing migration. Editing a migration that may already be applied "
                                 "on dev/prod causes schema drift; add a new NNNN_*.sql instead "
                                 "(backend-db-migration-safe). Approve only if this migration was never applied.")
    if norm.endswith("/docker-compose.artifacts.yml"):
        pre_tool_decision("ask", f"{r} defines the dev-server stack. Confirm the change and keep secrets as "
                                 "${VAR:?} references, never literals.")
sys.exit(0)
