# Token Replay Semantics

## Architecture Reality: Stateless Token
- `TOKEN_SINGLE_USE = false`
- `TRUE_IDEMPOTENCY_IMPLEMENTED = false`
- `NEW_IMPORT_SESSION_TABLE_REQUIRED = false`
- `IMPORT_IDEMPOTENCY_TABLE_CREATED = false`
- `PROCESS_LOCAL_REPLAY_CACHE_CREATED = false`

There is no import session table, token blacklisting table, or in-memory cache. Token verification is completely stateless.

## Same Token + Same Target Replay
- **Behavior**: `SAME_TOKEN_SAME_TARGET_REPLAY = FIRST_201_SECOND_409`
- First request creates the new template root and draft v1, returning HTTP 201 Created.
- Second identical request encounters `target_type_id` conflict (via `TypeExists` precheck and DB unique index) and returns HTTP 409 Conflict.
- Guarantees:
  - No second root
  - No second version
  - No duplicate blocks
  - No duplicate audit event
  - This is **Duplicate Resource Protection**, not true idempotency.

## Same Token + Different Target ID
- **Behavior**: `TOKEN_REPLAY_DIFFERENT_TARGET_ID_BEHAVIOR = ALLOWED_BY_CONTRACT`
- The validation token binds the portable template payload hash, not the target identity.
- As long as the token is within its 15-minute TTL and the actor matches, the same validated payload can be materialized into distinct target IDs (e.g. creating two templates based on the same imported schema with different target type IDs).
- Proven in test `TestTemplateImportConfirm_ReplaySemantics/same_token_different_target_id_allowed_by_contract`.

## Failed Confirm Does Not Consume Token
- When a materialization fails due to transient database failure or network drop, the stateless token remains valid until its 15-minute TTL expires.
- The client can retry the Confirm request with the same token and target after the issue resolves.
- Proven in test `TestTemplateImportConfirm_RollbackAndZeroPartialWrite`.
