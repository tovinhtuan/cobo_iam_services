# CoBo Template Builder GPT

This package configures a Custom GPT that turns a user's raw report description
into a CMS template-import JSON file.  It is intentionally an authoring tool,
not a CMS administrator.

## What is ready

- `instructions.md`: paste this into the Custom GPT **Instructions** field.
- `knowledge-manifest.md`: upload the listed source files to GPT Knowledge.
- `cobo-template-builder-action.openapi.yaml`: contract for the future
  validate-only Action.

## Safe rollout

1. Create the GPT with file upload and Code Interpreter enabled.
2. Upload the Knowledge files in `knowledge-manifest.md`.
3. Paste `instructions.md` into Instructions.
4. Run it in offline mode first: users download JSON and validate/import it in
   the CMS UI.
5. Add the Action only after the dedicated validation gateway is deployed and
   secured. Do **not** point it at the existing CMS import endpoint.

The GPT must never call confirm, publish, activate, catalog-mutation, or email
operations.  A successful validation result means only that the file is ready
for the CMS import screen; it does not create, release, or activate a template.

## Operational ownership

The final source of truth remains the Cobo backend validator.  The existing
CMS UI owns department mapping, confirmation, publishing, activation, and all
audit/lifecycle effects.
