# Reusable Task Updates

## 2026-04-27 - Mandatory Prompt Policy (2 repos)

- task type: understand
- objective: enforce a mandatory reusable prompt policy for all tasks touching `cobo_web_design` and `cobo_iam_services`
- discovered/implemented:
  - added a mandatory policy block to `docs/ai-cache/README.md` to require:
    - read `docs/ai-cache/README.md` + reusable cache first
    - conflict priority order
    - skill selection and mandatory `integration-cross-repo` for cross-repo tasks
    - contract-first for new features
    - premerge review + fresh Docker rebuild after code changes
    - reusable update writeback into `docs/ai-cache/` after each task
- affected repos/files/modules:
  - `cobo_web_design/docs/ai-cache/README.md`
  - `cobo_iam_services/docs/ai-cache/README.md`
- important contracts/behaviors/constraints/decisions:
  - this policy is process-level guidance; it does not change runtime product behavior
  - do not overwrite source-of-truth content unless task explicitly requires
- build/verification result:
  - no runtime code changed; docker build not required for this documentation-only update
- remaining gaps/risks/next steps:
  - ensure future prompts consistently include/adhere to this mandatory block
