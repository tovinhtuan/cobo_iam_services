---
description: Sửa bug Cobo theo wf-bugfix (tái hiện trước, root cause có bằng chứng, test hồi quy)
argument-hint: <mô tả bug / request id / bước tái hiện>
---

Run the `wf-bugfix` skill. Do not change production code before a
reproduction exists (GATE R) unless I explicitly accept a code-reading-only
fix. Report root cause with evidence, then fix, verify (GATE V) and assess
blast radius.

Bug: $ARGUMENTS
