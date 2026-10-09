#!/usr/bin/env python3
"""Detect and repair Vietnamese mojibake (UTF-8 bytes mis-decoded as cp1252/latin-1/cp437).

  --scan PATH...        report files containing mojibake markers
  --print FILE          print best-effort repaired text to stdout
  --write-copy FILE     write repaired text to FILE.fixed (never overwrites FILE)
Stdlib only.
"""
import os
import re
import sys

MARKERS = re.compile(
    r"(Ã[\u0080-ÿ]|Ä[\u0080-ÿ]|Æ[\u0080-ÿ]|áº|á»|Ä‘|Ä'|Â |â€|"
    r"[╗╝╚╔║═├┤┬┴┼█▓▒░][^\n]{0,3}[╗╝╚╔║═├┤┬┴┼█▓▒░]|ß║|ß╗|─æ|├í|├ú|├┤|╞░)")
SKIP_DIRS = {".git", "node_modules", ".venv", "dist", "__pycache__", "coverage"}
TEXT_EXT = {".md", ".txt", ".ts", ".tsx", ".js", ".mjs", ".go", ".sql", ".json", ".yml", ".yaml",
            ".html", ".tmpl", ".py", ".ps1", ".sh", ".env", ".example", ".csv"}


def score(text):
    return len(MARKERS.findall(text))


def repair(text):
    best, best_enc, best_score = text, None, score(text)
    for enc in ("cp1252", "latin-1", "cp437"):
        try:
            cand = text.encode(enc, errors="strict").decode("utf-8", errors="strict")
        except (UnicodeEncodeError, UnicodeDecodeError):
            # line-by-line fallback for mixed files
            lines, changed = [], False
            for line in text.splitlines(keepends=True):
                try:
                    fixed = line.encode(enc).decode("utf-8")
                    if score(fixed) < score(line):
                        lines.append(fixed)
                        changed = True
                        continue
                except (UnicodeEncodeError, UnicodeDecodeError):
                    pass
                lines.append(line)
            if not changed:
                continue
            cand = "".join(lines)
        s = score(cand)
        if s < best_score:
            best, best_enc, best_score = cand, enc, s
    return best, best_enc


def iter_files(paths):
    for p in paths:
        if os.path.isfile(p):
            yield p
            continue
        for d, dirs, files in os.walk(p):
            dirs[:] = [x for x in dirs if x not in SKIP_DIRS]
            for f in files:
                if os.path.splitext(f)[1].lower() in TEXT_EXT:
                    yield os.path.join(d, f)


def read(p):
    with open(p, "r", encoding="utf-8", errors="replace") as fh:
        return fh.read()


def main(argv):
    if len(argv) < 2:
        print(__doc__)
        return 2
    mode, args = argv[0], argv[1:]
    if mode == "--scan":
        hits = 0
        for p in iter_files(args):
            n = score(read(p))
            if n:
                hits += 1
                _, enc = repair(read(p))
                print(f"{p}: {n} marker(s){' (repairable via ' + enc + ')' if enc else ''}")
        print(f"\n{hits} file(s) with mojibake markers")
        return 1 if hits else 0
    if mode in ("--print", "--write-copy"):
        p = args[0]
        fixed, enc = repair(read(p))
        if enc is None:
            print(f"No repair found for {p}", file=sys.stderr)
            return 1
        if mode == "--print":
            sys.stdout.write(fixed)
        else:
            out = p + ".fixed"
            with open(out, "w", encoding="utf-8", newline="") as fh:
                fh.write(fixed)
            print(f"Wrote {out} (decoded via {enc}); review and diff before replacing.")
        return 0
    print(__doc__)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
