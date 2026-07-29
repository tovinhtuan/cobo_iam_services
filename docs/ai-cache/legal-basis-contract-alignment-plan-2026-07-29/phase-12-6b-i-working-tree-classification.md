# Phase 12.6B-I — Working tree classification

Baseline before safety commit: a07fd4d
Safety commit: 2d9a5b7

| path | origin | purpose | 12.6A safety? | 12.6B-I? | decision |
| --- | --- | --- | --- | --- | --- |
| cmd/legal-basis-inventory/main.go | 12.6A uncommitted | docker-dev RO + allowlisted open | Yes | Support | COMMITTED safety chore |
| internal/.../sql_allowlist.go | 12.6A | SQL allowlist interceptor | Yes | Support | COMMITTED |
| internal/.../sql_allowlist_test.go | 12.6A | allow/forbid tests | Yes | No | COMMITTED |

After safety commit: no unresolved dirty inventory files.
