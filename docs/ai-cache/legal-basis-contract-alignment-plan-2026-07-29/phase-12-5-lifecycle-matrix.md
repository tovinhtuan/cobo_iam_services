# Phase 12.5 — Lifecycle ownership matrix (audit)

| Operation | Entry point | Service | Repository | Source aggregate | Target aggregate | Current legal_bases behavior | Current ID behavior | Projection behavior | Gap | Class | Owner | Action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Create draft (v1) | PUT upsert | UpsertTypeVersion | mysql/inmemory UpsertTypeVersion | — | New type v1 | Persist client/sanitize/project | Fill blank IDs | Flag-gated | Needs ID-preserve tests | B | BE | Tests only |
| Update same draft (omit) | PUT upsert | same | Preserve reload when overwriteDraft | Same draft | Same draft | Reload DB JSON | Preserve | Keep / client flat | Missing integration assert | A+B | BE | Tests |
| Update same draft (provided) | PUT upsert | ResolveLegalBasisWrite | UPDATE JSON | Same draft | Same draft | Replace with payload | Preserve non-empty client IDs; fill blank | Project when flag ON | OK | A+B | BE | Tests |
| Publish (company) | lifecycle | TransitionCompany… | review_status only | Same | Same | Untouched | Untouched | Untouched | — | A | BE | No-op code |
| Activate | ActivateTypeVersion | same | activate SQL | Same version row | Active pointer | Untouched | Untouched | Untouched | — | A | BE | No-op / optional test |
| Archive / read-back | Archive + Get detail | CmsArchive / GetType* | status / SELECT | Same rows | Same | Untouched content; read compat | Untouched | Read project/synthesize | — | A | BE | No-op |
| **Create new version** | PUT upsert | same | INSERT MAX+1 | Prior max version | New version | **Omit → `[]` wipe** | No regen | Flat may persist without structured | **Data loss + no ID regen** | **C** | BE | **Fix** |
| Clone | — | — | — | — | — | N/A | N/A | N/A | Path absent | N/A | Product | Document only |
| Global→company copy | — | — | — | — | — | N/A | N/A | N/A | Path absent | N/A | Product | Document only |
| Company create | POST company | CreateCompanyTemplate | INSERT flat only | — | Company v1 | No structured write | N/A | Flat only | Public DTO locked | A | BE | No-op (scope) |
| Company edit (flat) | PATCH company | UpdateCompanyTemplate | UPDATE flat only | Company active | Same | JSON untouched | N/A | Flat updates; structured wins on read if present | Divergence risk deferred | A | BE | No-op / note |

## No-op / minimal-fix gate

| Class | Operations |
| --- | --- |
| A no-op | Publish, Activate, Archive, Company create/edit (public), Clone N/A, Global→company N/A |
| B tests | Same-draft preserve; activate isolation assert |
| C fix | **New version** deep-copy from prior version when preserve OR provided (regen IDs); re-project when structured exists |
| D conflict | None requiring STOP after N/A decision for missing clone/copy |
