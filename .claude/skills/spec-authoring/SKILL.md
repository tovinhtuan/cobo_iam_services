---
name: spec-authoring
description: Dùng khi viết hoặc cập nhật tài liệu đặc tả SPEC-*.md, plan/todo trong tasks/ cho Cobo - objective, hiện trạng, data model/migration, API contract, UI state machine, security invariants, known gaps, testing strategy, boundaries, rollout theo phase với Phase 0 contract lock.
---

# Spec Authoring

## When to use
- A new feature needs a written spec before coding.
- An existing `SPEC-*.md` must reflect a decision or change.
- Creating `tasks/<topic>-plan.md` + `tasks/<topic>-todo.md`.

## Conventions (observed in repo)
- Specs live at the web repo root as `SPEC-<topic>.md`
  (`SPEC-account-management.md`, `SPEC-workflow-permission.md`, ...).
- Plans/todos in `tasks/`: phased checkboxes with CHECKPOINT commands; Phase 0
  is "Contract Lock (HARD STOP)".
- Template: `references/spec-template.md`.

## Workflow
1. Read existing specs, ai-cache notes and code for the area; record what
   exists vs is missing (with file paths).
2. Fill the template; keep each section concrete (endpoints, fields, codes,
   states). Mark assumptions and open questions explicitly.
3. Cross-check with skills: `cobo-admin-role-guard` (roles),
   `api-compatibility-rolling-deploy` (compat), `backend-db-migration-safe` [iam]
   (schema), `cobo-domain-cbtt` (legal basis).
4. Write the plan/todo with phases small enough to verify independently;
   each phase ends with exact verification commands.
5. Keep Vietnamese text UTF-8 (`vi-text-encoding`).

## Guardrails
- A spec describes decisions; don't present guesses as facts.
- Don't invent endpoints that contradict existing code without flagging it.
- Update the spec when implementation deviates.
