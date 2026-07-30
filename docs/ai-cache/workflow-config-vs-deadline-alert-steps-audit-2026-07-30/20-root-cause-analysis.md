# 20 — Root cause analysis

## Decision tree path

**A** CMS global config ≠ effective → dual SoT / RC-4.
**B** Effective == snapshot → no materializer source defect.
**C** Snapshot == alert steps API → no alert join defect for steps.
**D** Detail UI fails on tasks 500 → separate FE/BE tasks path issue (contributing UX blocker).
**E** List shows current only → EXPECTED_RUNTIME_REPRESENTATION.

## Primary: RC-4
CMS Workflow Configuration (global versioning) empty; effective/materialize/alert use `global_template` / `enterprise_workflow` portal path.

## Contributing
- RC-2: list card runtime-only current step
- RC-8/partial: detail page fails when tasks endpoint 500 (not wrong step mapping; blocked render)
- OPEN: whether operators should see empty global builder as “no workflow” while portal has 4 steps

## Not confirmed
RC-5/6 materializer wrong source/version — **rejected** (source matches effective).
RC-9 FE order bug — **rejected** for steps API.
RC-1 immutable snapshot vs newer CMS — N/A (no newer global CMS version).
