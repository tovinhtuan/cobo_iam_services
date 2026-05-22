# Inventory Index

> Updated: 2026-05-22
> Scope: navigation index for project inventory and traceability documents in `docs/`

## Purpose

This is the entrypoint for the current project documentation set that inventories features, routes, modules, and lifecycles across FE and BE.

## Reading Order

1. Start with the full picture:
   - [Current Feature And Flow Inventory](./current-feature-flow-inventory.md)
2. Then pick the lens you need:
   - route-to-endpoint tracing
   - implementation status
   - backend ownership
   - lifecycle walkthrough
   - CMS-only or Enterprise Admin-only scope

## Choose By Question

| If you want to know... | Read this file |
|---|---|
| What is currently designed across the whole workspace? | [current-feature-flow-inventory.md](./current-feature-flow-inventory.md) |
| Which FE route maps to which BE endpoint? | [fe-route-to-be-endpoint-matrix.md](./fe-route-to-be-endpoint-matrix.md) |
| What is implemented, partial, or only planned? | [feature-implementation-status.md](./feature-implementation-status.md) |
| Which backend module owns which surface and endpoints? | [backend-module-inventory.md](./backend-module-inventory.md) |
| What is the end-to-end lifecycle for auth, disclosure, reminder, or ops? | [lifecycle-flow-inventory.md](./lifecycle-flow-inventory.md) |
| What exists only in the CMS/platform admin surface? | [cms-feature-inventory.md](./cms-feature-inventory.md) |
| What exists only in the tenant Enterprise Admin surface? | [enterprise-admin-feature-inventory.md](./enterprise-admin-feature-inventory.md) |

## Document Set

### Core workspace inventory

- [current-feature-flow-inventory.md](./current-feature-flow-inventory.md)
  Single broad inventory of features and flows across `cobo_iam_services` and `cobo_web_design`.

### Traceability and status

- [fe-route-to-be-endpoint-matrix.md](./fe-route-to-be-endpoint-matrix.md)
  Best starting point for screen-to-API tracing.

- [feature-implementation-status.md](./feature-implementation-status.md)
  Best starting point for scoping gaps, rollout readiness, and review status.

### Ownership and lifecycle views

- [backend-module-inventory.md](./backend-module-inventory.md)
  Best starting point when changing backend code by module ownership.

- [lifecycle-flow-inventory.md](./lifecycle-flow-inventory.md)
  Best starting point when reasoning about end-to-end domain behavior.

### Focused product surfaces

- [cms-feature-inventory.md](./cms-feature-inventory.md)
  CMS-only surface and contracts.

- [enterprise-admin-feature-inventory.md](./enterprise-admin-feature-inventory.md)
  Enterprise Admin-only surface and contracts.

## Suggested Usage

- For onboarding:
  - read `current-feature-flow-inventory.md`
  - then `inventory-index.md`
  - then whichever focused doc matches the area you will touch
- For implementation:
  - start with the route matrix or backend-module inventory
- For product review:
  - start with implementation status and lifecycle inventory
