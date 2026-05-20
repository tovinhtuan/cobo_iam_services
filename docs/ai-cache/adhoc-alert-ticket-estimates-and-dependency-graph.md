# Ad-Hoc Alert Proposal Tickets — Estimates And Dependency Graph

## Mục tiêu

Bổ sung cho Jira-ready checklist:

- estimate `S/M/L`
- dependency graph giữa các ticket
- critical path khuyến nghị

Quy ước estimate:

- `S`: 0.5 - 1.5 dev day
- `M`: 2 - 4 dev days
- `L`: 4 - 8 dev days

Estimate dưới đây là estimate kỹ thuật tương đối cho một team đã có context codebase.

---

## Ticket Estimates

| Ticket | Size | Why |
|---|---|---|
| `P0-1` Align deadline contract | `M` | chạm FE form, FE mapper, BE contract, persistence, docs, tests |
| `P0-2` First-class title/description | `L` | nhiều khả năng cần migration + DTO + mapper + backward compatibility |
| `P0-3` Harden admin-approve consistency | `L` | cần investigation, sequence review, idempotency/retry tests, có thể đụng nhiều lớp |
| `P0-4` Fix/clarify downstream actor attribution | `M` | scope hẹp hơn P0-3 nhưng vẫn chạm downstream semantics + tests |
| `P0-5` Backend validation parity | `S` | chủ yếu BE service/handler validation + tests, phụ thuộc semantics đã chốt |
| `P1-1` Real draft editing | `L` | phải thêm API update draft + hydrate form + state guard + FE flow |
| `P1-2` Gate create CTA by propose permission | `S` | FE-only gating + test |
| `P1-3` Show process controller display info | `M` | cần enrich response hoặc mapping + UI updates |
| `P1-4` Clean deprecated contract artifacts | `M` | inventory + cleanup an toàn cross FE/docs/BE contract |
| `P1-5` Review audit/notification coverage | `S` | review/ADR style nếu chưa implement runtime |
| `P2-1` Refresh BA-facing docs | `S` | docs-only |
| `P2-2` Refresh API contract docs | `S` | docs-only |
| `P2-3` Clarify product language proposal vs final alert | `S` | docs/content/product alignment |

---

## Dependency Graph

### Direct dependencies

- `P0-5` depends on `P0-1`
  - vì backend validation parity cần semantics đúng của deadline field
- `P0-2` should not start before `P0-1` is settled
  - để tránh cleanup contract xong rồi lại đổi payload shape vì semantics deadline
- `P0-3` benefits from `P0-4`
  - actor attribution clarity giúp review đúng downstream side effects
- `P1-1` depends on `P0-1` and partially on `P0-2`
  - nếu draft edit được làm trước khi contract cleanup, sẽ dễ phải sửa lại payload/form lần hai
- `P1-3` optionally depends on `P0-4`
  - nếu business actor/process controller semantics đổi, UI display contract có thể đổi theo
- `P1-4` should happen after `P0-1`, `P0-2`, `P0-4`
  - vì cleanup artifact quá sớm dễ remove nhầm compatibility shim
- `P2-1`, `P2-2`, `P2-3` depend on resolved/accepted outcomes from `P0/P1`

### Graph view

```text
P0-1 --> P0-5
P0-1 --> P0-2
P0-1 --> P1-1

P0-4 --> P0-3
P0-4 --> P1-3

P0-2 --> P1-1
P0-1 --> P1-4
P0-2 --> P1-4
P0-4 --> P1-4

P0-* / P1-* --> P2-1
P0-* / P1-* --> P2-2
P0-* / P1-* --> P2-3
```

---

## Critical Path

### Phase A — Contract correctness

1. `P0-1`
2. `P0-5`

### Phase B — Downstream correctness

3. `P0-4`
4. `P0-3`

### Phase C — Contract cleanup / product completeness

5. `P0-2`
6. `P1-2`
7. `P1-1`
8. `P1-3`
9. `P1-4`
10. `P1-5`

### Phase D — Documentation alignment

11. `P2-1`
12. `P2-2`
13. `P2-3`

---

## Parallelization Opportunities

### Can run in parallel after `P0-1` is settled

- `P0-5`
- planning/design work for `P0-2`
- FE-only `P1-2`

### Can run in parallel after actor decision is settled

- `P0-3`
- `P1-3`

### Docs can be parallelized late

- `P2-1`
- `P2-2`
- `P2-3`

---

## Recommended Sprint Framing

### Sprint 1

- `P0-1`
- `P0-5`
- `P1-2`

### Sprint 2

- `P0-4`
- `P0-3`

### Sprint 3

- `P0-2`
- `P1-1`
- `P1-3`

### Sprint 4

- `P1-4`
- `P1-5`
- `P2-*`

---

## Notes On Estimate Risk

### Tickets with highest variance

- `P0-3`
  - variance cao vì phụ thuộc reproduce/investigation depth
- `P0-2`
  - variance cao nếu cần backward compatibility lâu hơn dự kiến
- `P1-1`
  - variance cao nếu edit draft kéo theo refactor create page lớn hơn dự kiến

### Tickets with lowest variance

- `P1-2`
- `P2-1`
- `P2-2`
- `P2-3`
