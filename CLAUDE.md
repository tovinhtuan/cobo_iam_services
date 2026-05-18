# Claude Code — cobo_iam_services

## Agent Skills

Plugin agent-skills (Addy Osmani) đã cài tại `~/.claude/plugins/data/agent-skills-inline/`.
Mỗi session Claude Code CLI, meta-skill `using-agent-skills` được inject tự động qua SessionStart hook.

---

## Cách Dùng Skills Trong VSCode Chat

Nói tự nhiên, Claude tự chọn skill phù hợp:

```
dùng skill <tên-skill> cho <mô tả task>
```

Hoặc mô tả việc cần làm — Claude tự ánh xạ:

| Bạn nói | Skill được áp dụng |
|---------|-------------------|
| "build tính năng X, bắt đầu từ đầu" | `spec-driven-development` → `planning-and-task-breakdown` |
| "có bug ở hàm Y, giúp debug" | `debugging-and-error-recovery` |
| "thiết kế API endpoint Z" | `api-and-interface-design` |
| "review code trước khi merge" | `code-review-and-quality` |
| "đơn giản hóa đoạn code này" | `code-simplification` |
| "viết test cho hàm X" | `test-driven-development` |
| "chuẩn bị deploy" | `shipping-and-launch` |
| "xóa feature cũ / migrate" | `deprecation-and-migration` |
| "setup CI/CD pipeline" | `ci-cd-and-automation` |

---

## Cách Dùng Skills Trong Claude Code CLI

```bash
cd ~/go/src/myself/backend_api_cobo/cobo_iam_services
claude
```

Rồi dùng lệnh slash:

| Lệnh | Skill | Dùng khi |
|------|-------|---------|
| `/spec` | `spec-driven-development` | Bắt đầu feature/thay đổi lớn |
| `/plan` | `planning-and-task-breakdown` | Đã có spec, cần phân task |
| `/build` | `incremental-implementation` + TDD | Triển khai từng bước |
| `/test` | `test-driven-development` | Viết/chạy test |
| `/review` | `code-review-and-quality` | Trước khi merge |
| `/code-simplify` | `code-simplification` | Refactor/đơn giản hóa |
| `/ship` | `shipping-and-launch` | Deploy — chạy 3 agent song song |

---

## Workflow Chuẩn Theo Vòng Đời

### Feature mới
```
/spec   →  /plan  →  /build  →  /test  →  /review  →  /ship
```

### Sửa bug
```
(debugging-and-error-recovery)  →  /test  →  /review
```

### Refactor / xóa code cũ
```
/code-simplify  →  /test  →  /review
```

### API mới
```
(api-and-interface-design)  →  /spec  →  /build  →  /test
```

---

## 23 Skills — Tra Cứu Nhanh

### Define
| Skill | Khi nào dùng |
|-------|-------------|
| `interview-me` | Yêu cầu chưa rõ — hỏi từng câu để khai thác đúng nhu cầu |
| `idea-refine` | Có ý tưởng mơ hồ cần cụ thể hóa |
| `spec-driven-development` | Viết PRD trước khi code bất kỳ thứ gì |

### Plan
| Skill | Khi nào dùng |
|-------|-------------|
| `planning-and-task-breakdown` | Phân tách spec thành task nhỏ có tiêu chí hoàn thành |

### Build
| Skill | Khi nào dùng |
|-------|-------------|
| `incremental-implementation` | Implement từng lát mỏng, test xong mới tiếp |
| `test-driven-development` | Red → Green → Refactor |
| `api-and-interface-design` | Contract-first, Hyrum's Law, error semantics |
| `context-engineering` | Nạp đúng context vào session trước khi code |
| `source-driven-development` | Verify quyết định kỹ thuật từ tài liệu chính thức |
| `doubt-driven-development` | Phản biện quyết định khi stakes cao / code lạ |
| `frontend-ui-engineering` | Component, state, accessibility (dùng cho cobo_web_design) |

### Verify
| Skill | Khi nào dùng |
|-------|-------------|
| `debugging-and-error-recovery` | Tái hiện → khoanh vùng → sửa → bảo vệ |
| `browser-testing-with-devtools` | Debug UI với Chrome DevTools MCP |

### Review
| Skill | Khi nào dùng |
|-------|-------------|
| `code-review-and-quality` | 5-axis review: correctness/readability/architecture/security/performance |
| `code-simplification` | Xóa phức tạp thừa, giữ nguyên behavior |
| `security-and-hardening` | OWASP Top 10, input validation, secrets, auth |
| `performance-optimization` | Đo trước — Core Web Vitals / profiling |

### Ship
| Skill | Khi nào dùng |
|-------|-------------|
| `git-workflow-and-versioning` | Trunk-based, atomic commits |
| `ci-cd-and-automation` | Pipeline, quality gates, feature flags |
| `deprecation-and-migration` | Xóa code cũ an toàn, migrate người dùng |
| `documentation-and-adrs` | ADR, API docs — ghi lại *tại sao* |
| `shipping-and-launch` | Pre-launch checklist, staged rollout, rollback plan |

---

## 3 Agent Personas (dùng trong `/ship`)

| Agent | Vai trò |
|-------|---------|
| `code-reviewer` | Staff Engineer — 5-axis review |
| `test-engineer` | QA — coverage analysis, Prove-It pattern |
| `security-auditor` | OWASP Top 10, threat modeling |

---

## Nguyên Tắc Cốt Lõi (từ `using-agent-skills`)

1. **Luôn surface assumptions** trước khi implement — không điền assumption ngầm
2. **Stop khi gặp mâu thuẫn** — hỏi trước, không đoán
3. **Push back** khi approach có vấn đề rõ ràng — không yes-machine
4. **Enforce simplicity** — 100 dòng đủ thì không viết 1000 dòng
5. **Scope discipline** — chỉ chạm code được yêu cầu, không refactor ngoài scope
6. **Verify, không assume** — mọi task phải có evidence (test pass, build output)

---

## Kết Hợp Với docs/ai-cache

Repo này dùng hệ thống `docs/ai-cache/` (Cursor skill pack) song song với agent-skills:

- **agent-skills**: workflow engineering từ đầu đến cuối (spec → ship)
- **docs/ai-cache/**: context cụ thể của repo (architecture decisions, domain rules)

Khi làm task lớn: dùng skill `context-engineering` để load `docs/ai-cache/` vào context trước khi bắt đầu implement.
