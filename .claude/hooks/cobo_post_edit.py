#!/usr/bin/env python3
"""PostToolUse (Edit|Write|MultiEdit): fast feedback on the edited file.

Checks: high-confidence secrets, Vietnamese mojibake, gofmt, new migration missing
from run_dev_migrations.sh. Problems are reported to Claude via exit code 2.
"""
import importlib.util
import os
import re
import shutil
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from cobo_hooklib import project_dir, read_input, rel  # noqa: E402

data = read_input()
ti = data.get("tool_input", {}) or {}
path = ti.get("file_path") or ""
if not path:
    sys.exit(0)
base = project_dir(data)
abspath = path if os.path.isabs(path) else os.path.join(base, path)
if not os.path.isfile(abspath) or os.path.getsize(abspath) > 2_000_000:
    sys.exit(0)
norm = abspath.replace("\\", "/")
r = rel(abspath, base)
if "/.claude/" in norm or "/node_modules/" in norm:
    sys.exit(0)

problems = []


def load(mod_name, rel_path):
    p = os.path.join(base, ".claude", "skills", rel_path)
    if not os.path.isfile(p):
        return None
    spec = importlib.util.spec_from_file_location(mod_name, p)
    m = importlib.util.module_from_spec(spec)
    try:
        spec.loader.exec_module(m)
        return m
    except Exception:
        return None


try:
    with open(abspath, "r", encoding="utf-8", errors="replace") as fh:
        text = fh.read()
except OSError:
    sys.exit(0)

# 1. secrets (high-confidence patterns only)
sec = load("cobo_scan_secrets", "secrets-cors-cookie-review/scripts/scan_secrets.py")
if sec is not None:
    findings = []
    sec.scan_text(r, text, findings)
    strong = {"private-key", "aws-access-key", "google-api-key", "github-token", "openai-key", "slack-token",
              "jwt", "mysql-dsn-with-password", "url-with-password"}
    hits = [f for f in findings if f[2] in strong]
    if hits and not re.search(r"(_test\.go|\.test\.tsx?|/fixtures?/|\.example$)", norm):
        problems.append("Possible secret written: " + ", ".join(f"line {h[1]} [{h[2]}] {h[3]}" for h in hits[:5])
                        + ". Use env vars / ${VAR:?} references instead of literals (secrets-cors-cookie-review).")

# 2. mojibake
moj = load("cobo_fix_mojibake", "vi-text-encoding/scripts/fix_mojibake.py")
ext = os.path.splitext(norm)[1].lower()
if moj is not None and ext in moj.TEXT_EXT and "vi-text-encoding" not in norm:
    n = moj.score(text)
    if n:
        problems.append(f"{n} Vietnamese mojibake marker(s) in {r} (e.g. 'Ã', 'á»'). Rewrite the text as proper UTF-8 "
                        "(vi-text-encoding).")

# 3. gofmt
if ext == ".go" and shutil.which("gofmt"):
    try:
        out = subprocess.run(["gofmt", "-l", abspath], capture_output=True, text=True, timeout=15)
        if out.stdout.strip():
            problems.append(f"{r} is not gofmt-formatted. Run: gofmt -w {r}")
        if out.returncode != 0 and out.stderr.strip():
            problems.append(f"gofmt error: {out.stderr.strip()[:300]}")
    except Exception:
        pass

# 4. new migration listed in runner
m = re.search(r"/migrations/(\d{4}_[^/]+\.up\.sql)$", norm)
if m:
    runner = os.path.join(os.path.dirname(abspath), "run_dev_migrations.sh")
    if os.path.isfile(runner):
        with open(runner, encoding="utf-8", errors="replace") as fh:
            if m.group(1) not in fh.read():
                problems.append(f"{m.group(1)} is not in migrations/run_dev_migrations.sh; it will never be applied "
                                "on deploy. Append it to the MIGRATIONS list (backend-db-migration-safe).")

if problems:
    sys.stderr.write("Cobo post-edit checks:\n- " + "\n- ".join(problems) + "\n")
    sys.exit(2)
sys.exit(0)
