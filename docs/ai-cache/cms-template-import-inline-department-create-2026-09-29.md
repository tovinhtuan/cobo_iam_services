# CMS template import — inline department creation

Date: 2026-09-29  
Task type: implementation contract

## Objective

When validation identifies an unresolved workflow department, let an authorized Platform CMS user create one catalog department from the mapping step, then select that returned catalog code as the mapping target.

## Locked behavior

- The existing `POST /api/v1/platform/cms/workflow/template-departments` contract is reused; no new endpoint, migration, or change to the validate response.
- Creation is an explicit modal action, never automatic as a side effect of Validate. It writes to the global template-department catalog and is therefore visible to future global template workflows.
- The modal is prefilled with the source department name but remains editable. The backend generates the code when omitted and rejects duplicate normalized names.
- On HTTP 201, the UI inserts the returned catalog entry and maps the current source ID to its returned code immediately. The user may then Confirm using the existing import token if still valid.
- On 403, 409, or network failure, no mapping is set; the modal displays the error. A duplicate-name conflict requires selecting the already-existing catalog item after refresh.
- The import remains unconfirmed. Create-catalog is separately audited by the existing backend event `cms_template_department.create`.

## Authorization and safety

- Read of catalog remains `platform.cms.view` plus CMS read.
- Catalog creation remains server-enforced by `cms.template.write`; frontend visibility does not grant permission.
- Do not accept a client-supplied catalog code in this flow. The server slugifies and de-duplicates the code.

## Frontend state/test scope

- Add a `Tạo phòng ban mới` action per unresolved mapping and a scoped create modal.
- Cover successful create-and-select, create failure, and no automatic creation on initial Validate.
- Existing Confirm tests continue to prove that unmapped departments keep Confirm disabled.

## Out of scope

- No change to confirmation semantics, catalog deletion/edit, auto-provisioning, tenant-specific departments, or DEV deployment in this implementation.

## Implementation and verification

- Frontend: added an explicit per-mapping create action and modal in the CMS import flow. Successful creation selects the returned catalog code; failure leaves the mapping unresolved.
- Backend: unchanged; the existing create endpoint still generates/de-duplicates the code, enforces `cms.template.write`, and writes the existing audit event.
- `npm run test -- --run src/features/cms-core/templates/import/components/ImportMappingSection.test.tsx src/features/cms-core/templates/import/ImportTemplateScreen.test.tsx`: PASS (18 tests).
- `npm run build`: PASS.
- DEV/prod: not deployed or touched.

## Pre-merge review

- Critical: none.
- Important: a created department belongs to the global template catalog, not one tenant. The modal discloses this before the explicit create action; release notes should repeat the scope for CMS operators.
- Nice to have: add accent-insensitive duplicate-name matching if catalog governance requires names such as `Phòng Pháp chế` and `Phong Phap Che` to be treated as the same label.

## DEV deployment 2026-09-29

- Frontend-only deployment used `deploy-dev.ps1 -Mode fe -SkipTests`; no migrations, backend deployment, feature-flag, email, draft, publish, or activation action was run.
- The frontend build and transfer completed. The deployed web container reached `healthy`; DEV `/healthz` and `/readyz` both returned success.
- Browser interaction against the DEV host remained blocked by the in-app browser's saved control permission. Therefore the visible inline-create action, its failure handling, and mapping selection were not browser-smoked in this run.
- No catalog department was created during deployment verification. Production was not touched.
