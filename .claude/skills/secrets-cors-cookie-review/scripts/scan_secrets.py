#!/usr/bin/env python3
"""Heuristic secret scanner for Cobo repos (stdlib only).

Usage:
  python3 scan_secrets.py <path> [<path> ...]      # scan files/dirs
  git log -p --all | python3 scan_secrets.py --stdin   # scan git history
Values are always redacted in output. Exit code 1 if findings, else 0.
"""
import os
import re
import sys

SKIP_DIRS = {".git", "node_modules", ".venv", "venv", "__pycache__", ".pytest_cache",
             "vnstock.egg-info", ".playwright-mcp", "coverage"}
SKIP_EXT = {".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".pdf", ".zip", ".gz",
            ".xlsx", ".xls", ".docx", ".woff", ".woff2", ".ttf", ".mp4", ".har", ".sum", ".lock"}
MAX_BYTES = 2_000_000

PATTERNS = [
    ("private-key", re.compile(r"-----BEGIN (?:RSA |EC |OPENSSH |DSA |ENCRYPTED )?PRIVATE KEY-----")),
    ("aws-access-key", re.compile(r"\bAKIA[0-9A-Z]{16}\b")),
    ("google-api-key", re.compile(r"\bAIza[0-9A-Za-z_\-]{35}\b")),
    ("github-token", re.compile(r"\bgh[pousr]_[A-Za-z0-9]{36,}\b")),
    ("openai-key", re.compile(r"\bsk-(?:proj-)?[A-Za-z0-9_\-]{20,}\b")),
    ("slack-token", re.compile(r"\bxox[abprs]-[A-Za-z0-9\-]{10,}\b")),
    ("jwt", re.compile(r"\beyJ[A-Za-z0-9_\-]{10,}\.eyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}")),
    ("bearer-literal", re.compile(r"Bearer\s+[A-Za-z0-9._\-]{24,}")),
    ("mysql-dsn-with-password", re.compile(r"\b[\w.\-]+:([^@\s/'\"]{3,})@tcp\(")),
    ("url-with-password", re.compile(r"\b[a-z][a-z0-9+.\-]*://[^:/\s'\"]+:([^@\s'\"]{3,})@")),
    ("assigned-secret", re.compile(
        r"(?i)\b([A-Z0-9_]*(?:PASSWORD|PASSWD|SECRET|TOKEN|API_KEY|APIKEY|PRIVATE_KEY|DSN)[A-Z0-9_]*)\b"
        r"\s*[:=]\s*['\"]?([^'\"\s#,;]{6,})")),
    ("js-password-literal", re.compile(r"(?i)\b(password|passwd|pwd)\b\s*[:=]\s*['\"`]([^'\"`]{4,})['\"`]")),
]

# Values that are clearly placeholders.
PLACEHOLDER = re.compile(
    r"(?i)^(\$\{?[A-Z_]+\}?|<.*>|x{3,}|\*{3,}|changeme|change_me|example|placeholder|your[_-].*|my_.*|"
    r"dummy|test|secret|password|redacted|null|none|undefined|process\.env.*|os\.getenv.*|env\..*|"
    r"import\.meta\.env.*|cfg\..*|config\..*)$")


def redact(value: str) -> str:
    v = value.strip()
    if len(v) <= 6:
        return "***"
    return v[:2] + "***" + v[-2:]


def scan_text(label, text, findings):
    for lineno, line in enumerate(text.splitlines(), 1):
        if len(line) > 4000:
            line = line[:4000]
        for name, rx in PATTERNS:
            for m in rx.finditer(line):
                value = m.group(m.lastindex) if m.lastindex else m.group(0)
                if name in ("assigned-secret", "js-password-literal") and PLACEHOLDER.match(value.strip()):
                    continue
                findings.append((label, lineno, name, redact(value)))


def iter_files(root):
    if os.path.isfile(root):
        yield root
        return
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        for f in filenames:
            p = os.path.join(dirpath, f)
            if os.path.splitext(f)[1].lower() in SKIP_EXT:
                continue
            try:
                if os.path.getsize(p) > MAX_BYTES:
                    continue
            except OSError:
                continue
            yield p


def main(argv):
    findings = []
    if "--stdin" in argv:
        scan_text("<stdin>", sys.stdin.read(), findings)
    else:
        targets = [a for a in argv if not a.startswith("--")] or ["."]
        for t in targets:
            for p in iter_files(t):
                try:
                    with open(p, "r", encoding="utf-8", errors="ignore") as fh:
                        scan_text(p, fh.read(), findings)
                except OSError:
                    pass
    for label, lineno, name, val in findings:
        print(f"{label}:{lineno}: [{name}] {val}")
    print(f"\n{len(findings)} potential secret(s). Review each; placeholders and test fixtures may be false positives.")
    return 1 if findings else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
