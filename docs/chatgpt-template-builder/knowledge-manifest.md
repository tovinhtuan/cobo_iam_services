# Knowledge manifest

Upload these files to the Custom GPT's Knowledge area. Keep the files
versioned and free of secrets, tokens, real-user data, and production exports.

1. `../schema/template-import-v1.schema.json` — exact JSON structure.
2. `../../internal/disclosure/app/artifacts/cobo-template-import-guide-v1.0.md`
   — user-facing field rules and error remedies.
3. `../schema/template-import-v1.example.json` — canonical valid example.
4. Approved valid and invalid fixtures, if and only if they are sanitized.
5. A maintained error-code reference generated from validator tests/docs.
6. A portable department mapping policy: code/name conventions only, never
   tenant UUIDs or credentials.

Do not upload migrations, `.env` files, API captures, access tokens, database
dumps, production logs, or the CMS validation-token implementation.

## Refresh rule

Whenever the schema, normalizer, validator, or guide changes, refresh all of
items 1–3 together and test the GPT against a valid periodic, valid irregular,
bad JSON, invalid periodicity, missing irregular deadline, and unresolved
department case.
