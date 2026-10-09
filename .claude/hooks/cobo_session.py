#!/usr/bin/env python3
"""SessionStart: inject Cobo context + snapshot working tree.
Stop: if code changed this session but docs/ai-cache did not, remind once.

Disable the Stop reminder with env COBO_HOOK_STOP=0.
"""
import json
import os
import subprocess
import sys
import tempfile

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from cobo_hooklib import project_dir, read_input  # noqa: E402

CODE_EXT = {".go", ".ts", ".tsx", ".js", ".mjs", ".sql", ".yml", ".yaml", ".sh", ".ps1", ".py", ".json", ".css",
            ".html", ".tmpl"}
STATE_DIR = os.path.join(tempfile.gettempdir(), "cobo-claude-hooks")


def changed_snapshot(base):
    try:
        out = subprocess.run(["git", "status", "--porcelain", "--untracked-files=all"], cwd=base, capture_output=True, text=True, timeout=20)
    except Exception:
        return {}
    snap = {}
    for line in out.stdout.splitlines():
        p = line[3:].split(" -> ")[-1].strip().strip('"')
        full = os.path.join(base, p)
        try:
            st = os.stat(full)
            snap[p] = [st.st_mtime, st.st_size]
        except OSError:
            snap[p] = [0, 0]
    return snap


def state_path(session_id):
    os.makedirs(STATE_DIR, exist_ok=True)
    return os.path.join(STATE_DIR, f"{session_id or 'unknown'}.json")


def start(data, base):
    sid = data.get("session_id", "")
    snap = changed_snapshot(base)
    try:
        with open(state_path(sid), "w") as fh:
            json.dump(snap, fh)
    except OSError:
        pass
    try:
        branch = subprocess.run(["git", "rev-parse", "--abbrev-ref", "HEAD"], cwd=base, capture_output=True,
                                text=True, timeout=5).stdout.strip()
    except Exception:
        branch = "?"
    ctx = (
        f"Cobo repo '{os.path.basename(base)}' on branch '{branch}' with {len(snap)} uncommitted path(s) before this "
        "session. Rules: read docs/ai-cache/README.md first (if it looks garbled run "
        "`python3 .claude/skills/vi-text-encoding/scripts/fix_mojibake.py --print docs/ai-cache/README.md`); start "
        "non-trivial work with the cobo-task-workflow skill or a workflow (wf-feature, wf-bugfix, wf-risk-review, "
        "wf-pr-review, wf-release; commands /task /feature /bugfix /risk-review /pr-review /release-check). Reviewer "
        "subagents: fe-security-reviewer, be-security-reviewer, admin-role-reviewer, perf-reliability-reviewer, "
        "cache-versioning-reviewer, api-compat-reviewer, secrets-cors-reviewer. No push/deploy/destructive commands "
        "without an explicit user request."
    )
    print(json.dumps({"hookSpecificOutput": {"hookEventName": "SessionStart", "additionalContext": ctx}}))


def stop(data, base):
    if os.environ.get("COBO_HOOK_STOP", "1") == "0" or data.get("stop_hook_active"):
        return
    sid = data.get("session_id", "")
    try:
        with open(state_path(sid)) as fh:
            before = json.load(fh)
    except Exception:
        return  # no baseline -> do not guess
    now = changed_snapshot(base)
    touched = [p for p, v in now.items() if before.get(p) != v]
    code = [p for p in touched if not p.startswith("docs/ai-cache") and
            (os.path.splitext(p.rstrip("/"))[1].lower() in CODE_EXT or os.path.basename(p) == "Makefile")]
    cache = [p for p in touched if p.startswith("docs/ai-cache")]
    if code and not cache:
        # remember so we remind only once per change set
        try:
            with open(state_path(sid), "w") as fh:
                json.dump(now, fh)
        except OSError:
            pass
        reason = ("Cobo reminder: code changed this session (" + ", ".join(code[:8]) +
                  (" ..." if len(code) > 8 else "") + ") but nothing under docs/ai-cache. Per repo rules: (1) report "
                  "verification commands and results or BLOCKED:, (2) write the task summary with the "
                  "ai-cache-maintenance skill. If this was review-only, exploratory, or the user asked to skip, say so "
                  "briefly and finish.")
        print(json.dumps({"decision": "block", "reason": reason}))


if __name__ == "__main__":
    mode = sys.argv[1] if len(sys.argv) > 1 else ""
    d = read_input()
    b = project_dir(d)
    if mode == "start":
        start(d, b)
    elif mode == "stop":
        stop(d, b)
    sys.exit(0)
