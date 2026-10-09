"""Shared helpers for Cobo Claude Code hooks (stdlib only)."""
import json
import os
import re
import sys

SECRET_BASENAME = re.compile(r"(?i)(\.(pem|key|p12|pfx|jks|keystore)$|^id_(rsa|ed25519|ecdsa)(\.pub)?$)")
ENV_BASENAME = re.compile(r"(?i)^(\.env(\..+)?|.*\.local\.env|deploy-dev\.local\.env)$")
ENV_ALLOWED = re.compile(r"(?i)\.(example|sample|template)$")


def read_input():
    try:
        return json.load(sys.stdin)
    except Exception:
        return {}


def project_dir(data=None):
    return os.environ.get("CLAUDE_PROJECT_DIR") or (data or {}).get("cwd") or os.getcwd()


def is_secret_key_file(path):
    return bool(SECRET_BASENAME.search(os.path.basename(path or "")))


def is_env_file(path):
    b = os.path.basename(path or "")
    return bool(ENV_BASENAME.match(b)) and not ENV_ALLOWED.search(b)


def pre_tool_decision(decision, reason):
    """decision: allow | deny | ask"""
    print(json.dumps({
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": decision,
            "permissionDecisionReason": reason,
        }
    }))
    sys.exit(0)


def rel(path, base):
    try:
        return os.path.relpath(path, base)
    except ValueError:
        return path
