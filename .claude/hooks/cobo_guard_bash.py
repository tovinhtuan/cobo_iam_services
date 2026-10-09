#!/usr/bin/env python3
"""PreToolUse (Bash): ask before destructive/remote operations, deny dumping secret files."""
import os
import re
import shlex
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from cobo_hooklib import is_env_file, is_secret_key_file, pre_tool_decision, read_input  # noqa: E402

data = read_input()
cmd = ((data.get("tool_input") or {}).get("command") or "").strip()
if not cmd:
    sys.exit(0)

ASK_RULES = [
    (r"\bgit\s+push\b", "git push"),
    (r"\bgit\s+reset\s+--hard\b", "git reset --hard (discards work)"),
    (r"\bgit\s+clean\s+-[a-z]*f", "git clean -f (deletes untracked files)"),
    (r"\bgit\s+(filter-repo|filter-branch)\b", "git history rewrite"),
    (r"\bgit\s+rebase\b", "git rebase"),
    (r"\bgit\s+commit\b[^|;&]*--amend\b", "git commit --amend"),
    (r"\bgit\s+(tag|branch\s+-D|push\s+--delete)\b", "git tag/branch deletion"),
    (r"\bgit\s+(checkout|restore)\s+(--\s+)?\.\s*$", "discard all working-tree changes"),
    (r"\bdocker(-compose|\s+compose)\b[^|;&]*\bdown\b[^|;&]*\s-v\b", "docker compose down -v (deletes volumes/DB data)"),
    (r"\bdocker\s+(volume\s+(rm|prune)|system\s+prune|image\s+prune)", "docker volume/system prune"),
    (r"\bmake\s+[^|;&]*\b(deploy[\w-]*|dev-(up|down|restart|fix-web-perms)|push-migration)\b", "deploy / dev-server operation"),
    (r"(^|[\s/])deploy-dev\.(sh|ps1|cmd)\b", "deploy script"),
    (r"(^|[\s;&|])(ssh|scp|rsync|sshpass|plink|pscp)\s", "remote shell/copy to a server"),
    (r"(?i)\b(drop\s+(database|table|schema)|truncate\s+table|delete\s+from\s+\w+\s*(;|$))", "destructive SQL"),
    (r"\brm\s+-[a-z]*r[a-z]*f?\b[^|;&]*(migrations|docs/ai-cache|\.git\b|/\s*$)", "recursive delete of protected paths"),
]
for pattern, label in ASK_RULES:
    if re.search(pattern, cmd):
        pre_tool_decision("ask", f"Cobo guard: '{label}' needs explicit user approval for this exact operation "
                                 "(AGENTS.md: no push/deploy/destructive ops without a direct request).")

READERS = {"cat", "less", "more", "head", "tail", "bat", "nl", "xxd", "od", "strings", "base64"}
try:
    for segment in re.split(r"[|;&]+", cmd):
        tokens = shlex.split(segment, posix=True)
        if not tokens:
            continue
        prog = os.path.basename(tokens[0])
        if prog in READERS or (prog in {"grep", "rg"} and len(tokens) > 1):
            for t in tokens[1:]:
                if t.startswith("-"):
                    continue
                if is_secret_key_file(t):
                    pre_tool_decision("deny", f"Cobo guard: printing {t} would expose private key material.")
                if is_env_file(t):
                    pre_tool_decision("ask", f"Cobo guard: {t} may contain real credentials; output would land in "
                                             "the transcript. Prefer listing variable names only (e.g. cut -d= -f1).")
        if prog in {"printenv", "env"} and len(tokens) == 1:
            pre_tool_decision("ask", "Cobo guard: dumping the whole environment may print secrets.")
except ValueError:
    pass
sys.exit(0)
