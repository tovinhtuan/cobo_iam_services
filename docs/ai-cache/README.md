# Cursor Skill Pack for Cobo Repos

Pack n├áy gß╗ôm 2 bß╗Ö cß║Ñu h├¼nh:
- `cobo_web_design/.cursor/...`
- `cobo_iam_services/.cursor/...`

## C├ích d├╣ng
1. Copy th╞░ mß╗Ñc `.cursor` trong tß╗½ng repo v├áo ─æ├║ng project t╞░╞íng ß╗⌐ng.
2. Giß╗» c├íc `rules/*.mdc` ─æß╗â lu├┤n bß║¡t guardrails kiß║┐n tr├║c.
3. D├╣ng Agent trong Cursor v├á prompt theo skill t╞░╞íng ß╗⌐ng.
4. Vß╗¢i task lß╗¢n, bß║»t ─æß║ºu bß║▒ng `system-design-feature`.
5. Tr╞░ß╗¢c khi ho├án tß║Ñt, lu├┤n chß║íy `premerge-system-review`.

## Prompt mß║╖c ─æß╗ïnh n├¬n d├ín cho hß║ºu hß║┐t mß╗ìi c├óu hß╗Åi

D├╣ng prompt n├áy nh╞░ prompt khß╗ƒi ─æß║ºu gß║ºn nh╞░ mß╗ùi lß║ºn hß╗Åi Cursor.

```text
Use the relevant project skill for this task.
First identify the architectural boundary, affected layers, domain invariants, failure modes, validation strategy, and test scope before writing code.
Preserve backward compatibility unless explicitly asked otherwise.
Prefer minimal, reviewable diffs.
Do not skip loading/error/empty states on frontend.
Do not skip validation, authorization, idempotency, migration safety, or observability on backend.
Before marking the task done, run a pre-merge review and report risks, gaps, and verification steps.
```

## Prompt t├íi sß╗¡ dß╗Ñng theo tß╗½ng t├¼nh huß╗æng

### 1) Khi x├óy feature mß╗¢i tß╗½ ─æß║ºu

```text
Use system-design-feature first, then switch to the relevant repo-specific implementation skill.
Before coding, define the objective, user flow, domain invariants, API contract, UI states, data flow, failure modes, rollout approach, and test plan.
Only then implement with minimal and reviewable diffs.
```

### 2) Khi l├ám feature frontend trong `cobo_web_design`

```text
Use the frontend skill that best matches this task.
Follow the vertical slice structure: route -> screen -> feature components -> hooks/services -> types.
Do not mix route concerns, fetching concerns, and presentation concerns in one large file.
Handle loading, error, empty, success, disabled, and invalid-param states explicitly.
Add focused Vitest/testing-library coverage for the core user-visible behavior.
```

### 3) Khi l├ám feature backend trong `cobo_iam_services`

```text
Use the backend skill that best matches this task.
Keep boundaries clear: handler -> service/usecase -> repository -> external systems.
Define request/response contract, validation rules, authorization rules, transaction boundaries, cache impact, retry/idempotency considerations, and test matrix before coding.
Do not hide security, migration, or data consistency risks.
```

### 4) Khi sß╗¡a bug

```text
Use the relevant debugging or repo-specific skill.
First restate the symptom, expected behavior, actual behavior, likely root causes, and the most probable failure path from code.
Fix the root cause with the smallest safe change, then add regression protection and list any remaining uncertainty.
```

### 5) Khi review tr╞░ß╗¢c merge

```text
Run premerge-system-review.
Audit requirement coverage, architectural fit, frontend state completeness, validation completeness, API/contract consistency, auth/security risks, data consistency risks, migration/deployment risks, observability gaps, and missing regression tests.
Group findings into critical, important, and nice-to-have.
```

## Prompt si├¬u ngß║»n ─æß╗â ghim cß╗æ ─æß╗ïnh

Nß║┐u bß║ín muß╗æn mß╗Öt prompt ngß║»n h╞ín ─æß╗â d├╣ng li├¬n tß╗Ñc:

```text
Use the relevant project skill. Think in layers, define contracts first, handle failure modes explicitly, keep changes minimal, and do a system-level review before done.
```

## Mß║╣o d├╣ng thß╗▒c tß║┐
- Vß╗¢i task m╞í hß╗ô: lu├┤n bß║»t ─æß║ºu bß║▒ng prompt feature mß╗¢i.
- Vß╗¢i task chß╗ë chß║ím UI: d├╣ng prompt frontend.
- Vß╗¢i task auth/API/data: d├╣ng prompt backend.
- Vß╗¢i bug kh├│: d├╣ng prompt sß╗¡a bug.
- Vß╗¢i PR sß║»p xong: d├╣ng prompt review tr╞░ß╗¢c merge.

## Prompt khuy├¬n d├╣ng c┼⌐

```text
Use the relevant project skill for this task. Start by identifying the architectural boundary, domain invariants, failure modes, and validation strategy before coding. Prefer minimal, reviewable diffs and preserve backward compatibility unless explicitly asked otherwise.
```

```text
For this feature, use system-design-feature first, then use the repo-specific implementation skill. Do not start coding until you have listed API contract, UI states, data flow, edge cases, and test plan.
```

```text
Before marking this done, run premerge-system-review and report missing validation, missing UI states, contract mismatches, auth risks, data consistency risks, and regression gaps.
```

## Mandatory Prompt Requirement (2 repos)

├üp dß╗Ñng bß║»t buß╗Öc cho mß╗ìi prompt li├¬n quan `cobo_web_design` v├á/hoß║╖c `cobo_iam_services`:

1. Tr╞░ß╗¢c mß╗ìi b╞░ß╗¢c, ─æß╗ìc `docs/ai-cache/README.md` v├á to├án bß╗Ö context t├íi sß╗¡ dß╗Ñng trong `docs/ai-cache/`.
2. Thß╗⌐ tß╗▒ ╞░u ti├¬n khi c├│ xung ─æß╗Öt:
   - `docs/ai-cache/README.md`
   - c├íc file c├▓n lß║íi trong `docs/ai-cache/`
   - project rules
   - docs/pattern c┼⌐ trong repo
3. Chß╗ìn skill ph├╣ hß╗úp; nß║┐u task chß║ím cß║ú 2 repo th├¼ d├╣ng `integration-cross-repo`.
4. Vß╗¢i feature mß╗¢i: bß║»t buß╗Öc contract-first tr╞░ß╗¢c khi code (contract + request/response/error matrix + FE mapping + BE expectations + failure modes + rollout risks + validation plan).
5. Vß╗¢i task implement:
   - diff nhß╗Å, dß╗à review
   - kß║┐t th├║c bß║▒ng `premerge-system-review`
   - sau mß╗ùi cycle c├│ thay code phß║úi rerun fresh Docker build cho services bß╗ï ß║únh h╞░ß╗ƒng
   - kh├┤ng coi l├á xong cho tß╗¢i khi Docker build mß╗¢i nhß║Ñt ─æ├ú chß║íy v├á b├ío kß║┐t quß║ú
6. Vß╗¢i task ph├ón t├¡ch/review:
   - kh├┤ng sß╗¡a code nß║┐u ch╞░a ─æ╞░ß╗úc y├¬u cß║ºu explicit
   - ph├ón t├¡ch dß╗▒a tr├¬n code/docs thß╗▒c tß║┐, kh├┤ng phß╗Ång ─æo├ín
7. Sau mß╗ùi task (implement hoß║╖c understand), ghi t├│m tß║»t t├íi sß╗¡ dß╗Ñng v├áo `docs/ai-cache/` theo format ngß║»n, nhß║Ñt qu├ín:
   - task type
   - objective/question
   - implemented/discovered
   - affected repos/files/modules
   - contracts/behaviors/constraints/decisions
   - build/verification result (nß║┐u c├│)
   - remaining gaps/risks/next steps
