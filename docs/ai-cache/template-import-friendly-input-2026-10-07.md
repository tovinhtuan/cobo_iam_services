# Template import — friendly applicability input

## Scope

This change is limited to template JSON import validation and normalization. It
does not change routes, API envelopes, permissions, persistence schema, or the
confirmation-token contract.

## User-visible behavior

CMS authors may enter either canonical values or the Vietnamese display labels
in `template.applicability_rules`:

| Meaning | Canonical value | Accepted display label |
|---|---|---|
| Commercial | `commercial` | `Thương mại` |
| Service | `service` | `Dịch vụ` |
| Manufacturing | `manufacturing` | `Sản xuất` |
| Listed company | `listed` | `Công ty niêm yết` |
| Large public company | `large_public` | `Công ty đại chúng quy mô lớn` |
| Non-large public company | `non_large_public` | `Công ty đại chúng không phải quy mô lớn` |

If `applicable_sectors` is omitted during import, it defaults to all supported
sectors: commercial, service, and manufacturing. This import default does not
change direct CMS upsert validation semantics.

Known labels are normalized to canonical enum values before domain validation,
preview hashing, confirmation-token issuance, and persistence. Unknown values
remain invalid and still return `INVALID_APPLICABILITY_RULES`.

For periodic templates with `use_structure_deadline=false`,
`deadline_days` is the fixed/default rule and `deadline_by_structure` is not
required. The structure map is required only when
`use_structure_deadline=true`.

Reminder offsets in the import schema are relative to the deadline, so
`offsets_days: [-2, -1]` is normalized to the runtime contract
`days_before: [2, 1]` before confirmation. Invalid reminder values are now
reported during import validation instead of surfacing only during publish.

## Compatibility and rollback

Existing canonical JSON remains valid. The change is backend-only and can be
rolled back by restoring the previous frontend/backend artifact; no migration
or database rollback is required.

## Verification

- Focused disclosure applicability/import tests: pass.
- `go vet` for affected packages: pass.
- Full `go test ./...`: not green because of pre-existing/environment-sensitive
  failures outside this change, including Windows file-mode expectations,
  unrelated company access/config/integration tests, and notification template
  parity.
- Docker build: must be run before deployment; if the local Docker daemon is
  unavailable, deployment remains blocked.
