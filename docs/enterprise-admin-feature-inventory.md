# Enterprise Admin Feature Inventory

> Updated: 2026-05-22
> Scope: tenant/company administration under `/app/admin/*`

## Access Model

- FE route prefix: `/app/admin`
- Main permission gate:
  - `rbac.manage` for most enterprise admin capabilities
- Scope model:
  - company-scoped only
  - acts within the currently selected tenant/company context

## Route Inventory

| Route | Goal | Main backend surface |
|---|---|---|
| `/app/admin` | admin center shell | admin hub and related module APIs |
| `/app/admin/hub` | admin summary/landing | `GET /api/v1/admin/hub/summary` |
| `/app/admin/users` | users and memberships | company access membership/user endpoints |
| `/app/admin/roles` | roles and permissions | company role/permission admin endpoints |
| `/app/admin/rules` | authorization/rules builder | company rules endpoints |
| `/app/admin/audit` | audit and sessions | audit/session admin surfaces |
| `/app/admin/company` | own company profile | `GET /api/v1/admin/company`, `PATCH /api/v1/admin/company` |
| `/app/admin/departments` | department management | department CRUD/member assignment endpoints |
| `/app/admin/titles` | title management | title CRUD/assignment endpoints |

## Functional Groups

### 1. Admin hub

- tenant-level admin landing page
- summary metrics and shortcuts
- intended entry point into users, org structure, RBAC, rules, and audit

### 2. Users and memberships

- list company users/memberships
- create users
- invite users into company
- activate/deactivate or update membership state
- inspect linked roles, departments, and titles per membership

### 3. Roles and permissions

- inspect role matrix
- assign permissions to roles
- expose membership-role visibility and cross-check against effective access

### 4. Rules builder

- view/edit company-scoped rules
- authorization/resource-scope/workflow/notification rule handling
- validate or persist rule definitions through admin services

### 5. Audit and sessions

- inspect audit events for tenant-scoped actions
- inspect current sessions
- revoke sessions when needed

### 6. Company profile

- load current company profile
- update tenant-owned metadata
- intended for profile-level admin changes without crossing tenant boundaries

### 7. Departments

- list departments
- create/update/delete departments
- add/remove members from departments

### 8. Titles

- list titles
- create/update/delete titles
- assign/remove titles on memberships

## Related Backend Modules

| Module | Responsibility |
|---|---|
| `internal/companyaccess` | memberships, roles, departments, titles, admin endpoints |
| `internal/authorization` | permission/scope checks and effective access |
| `internal/audit` | audit event capture |
| `internal/iam` | sessions and current identity context |

## Current Status Notes

- Enterprise admin is a clearly designed and substantial product surface in both FE and BE.
- The strongest route-to-endpoint trace today is around:
  - admin hub
  - company profile
  - departments
  - titles
  - users/memberships via dedicated FE admin services
- Some admin capabilities are spread across multiple backend modules rather than living under a single monolithic admin handler, so cross-module tracing is required when changing behavior.
