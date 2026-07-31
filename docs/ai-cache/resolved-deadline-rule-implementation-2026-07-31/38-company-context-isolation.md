# Company-context isolation

## Exact identity
See `32-cross-layer-call-chain.md`.

## Proofs
| Gate | Evidence |
|------|----------|
| company in request | JWT Bearer after optional `switch-company`; BE `GetTypeDetail(Subject.CompanyID)` |
| company in FE load key | `companyContextKey` includes `selectedCompanyId` |
| typeId in request | path `/disclosure-types/{typeId}` |
| clear on switch | `setType(null)` before refetch |
| late response | `requestSeqRef` / simulated seq test ignores stale A after B switch |
| two-company | distinct rule fixtures A vs B |

## Verdict
Company isolation **PASS** — not FAIL_COMPANY_CONTEXT_CACHE.
