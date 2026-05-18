# Agent Skills — Hướng dẫn sử dụng

> 23 skills + 7 slash commands + 3 agent personas  
> Cài tại: `~/.claude/skills/` và `~/.claude/commands/`

---

## Slash Commands (7)

Gõ trực tiếp trong chat Claude Code để kích hoạt workflow tương ứng.

| Command | Chức năng | Dùng khi |
|---------|-----------|----------|
| `/spec` | Viết spec (PRD) trước khi code — hỏi mục tiêu, user, features, constraints rồi sinh `SPEC.md` | Bắt đầu feature mới hoặc yêu cầu chưa rõ |
| `/plan` | Breakdown spec thành danh sách task nhỏ, có thứ tự dependency và acceptance criteria | Đã có spec, cần lên kế hoạch implement |
| `/build` | Implement từng vertical slice nhỏ — code → test → verify → commit | Đang code, muốn đi từng bước an toàn |
| `/test` | Viết test theo TDD: Red → Green → Refactor, test pyramid, coverage | Cần proof code hoạt động đúng |
| `/review` | Code review 5 chiều: correctness, readability, architecture, security, performance | Trước khi merge PR |
| `/code-simplify` | Đơn giản hóa code — xóa complexity không cần thiết, giữ nguyên behavior | Code chạy đúng nhưng khó đọc/maintain |
| `/ship` | Checklist pre-launch: review code + security + test song song, rollback plan | Chuẩn bị deploy lên production |

---

## 23 Skills — Theo vòng đời phát triển

### META

#### `using-agent-skills`
**Chức năng:** Skill điều phối — đọc task đến rồi quyết định skill nào phù hợp nhất, theo flowchart SDLC.  
**Dùng khi:** Bắt đầu session hoặc không biết nên dùng skill nào.  
**Cách dùng:** Claude tự kích hoạt khi session bắt đầu (qua session hook). Hoặc hỏi: _"Tôi cần làm X, skill nào phù hợp?"_

---

### DEFINE — Làm rõ cần build gì

#### `interview-me`
**Chức năng:** Phỏng vấn từng câu một để khai thác yêu cầu thực sự, tránh assumption ngầm. Dừng khi đạt ~95% confidence.  
**Dùng khi:** Yêu cầu mơ hồ, user chưa rõ mình muốn gì, hoặc task có nhiều cách hiểu.  
**Cách dùng:**
```
Hãy interview-me về feature tôi muốn build
```

#### `idea-refine`
**Chức năng:** Biến ý tưởng thô thành proposal cụ thể qua divergent thinking (mở rộng) → convergent thinking (hội tụ).  
**Dùng khi:** Có ý tưởng rough nhưng chưa biết hình dạng cụ thể.  
**Cách dùng:**
```
Tôi có ý tưởng về [X], hãy giúp tôi refine nó
```

#### `spec-driven-development`
**Chức năng:** Viết PRD/spec đầy đủ trước khi code — gồm objective, commands, project structure, code style, testing strategy, boundaries. Sinh file `SPEC.md`.  
**Dùng khi:** Bắt đầu project/feature mới, change lớn, hoặc decision kiến trúc.  
**Cách dùng:** Gõ `/spec` — skill sẽ hỏi làm rõ rồi sinh spec.

---

### PLAN — Lên kế hoạch

#### `planning-and-task-breakdown`
**Chức năng:** Decompose spec thành task nhỏ, có acceptance criteria, dependency order, verification step. Mỗi task không chạm quá ~5 files.  
**Dùng khi:** Đã có spec, cần chia nhỏ thành unit có thể implement.  
**Cách dùng:** Gõ `/plan` hoặc:
```
Hãy breakdown spec này thành tasks nhỏ
```

---

### BUILD — Viết code

#### `incremental-implementation`
**Chức năng:** Implement từng thin vertical slice — code → test → verify → commit. Feature flags, safe defaults, rollback-friendly. Không implement nhiều thứ cùng lúc.  
**Dùng khi:** Bất kỳ change nào chạm hơn 1 file.  
**Cách dùng:** Gõ `/build` — hoặc nhắc _"implement từng slice nhỏ"_ khi giao task.

#### `test-driven-development`
**Chức năng:** TDD nghiêm ngặt — Red (viết test thất bại) → Green (code pass test) → Refactor. Test pyramid 80/15/5 (unit/integration/e2e). DAMP over DRY.  
**Dùng khi:** Implement logic, fix bug, thay đổi behavior.  
**Cách dùng:** Gõ `/test` hoặc:
```
Hãy viết test trước cho function X theo TDD
```

#### `context-engineering`
**Chức năng:** Tối ưu context cho agent — rules files, context packing, MCP integrations. Đảm bảo agent có đúng thông tin ở đúng thời điểm.  
**Dùng khi:** Bắt đầu session, output quality giảm, switch task, hoặc cần configure rules.  
**Cách dùng:**
```
Hãy setup context cho task implement X
```

#### `source-driven-development`
**Chức năng:** Ground mọi quyết định framework vào official documentation — verify, cite sources, flag unverified assumptions. Không code từ memory.  
**Dùng khi:** Dùng framework/library mà muốn code chính xác, có nguồn tham chiếu.  
**Cách dùng:**
```
Implement X theo source-driven approach, cite official docs
```

#### `doubt-driven-development`
**Chức năng:** Adversarial review trong khi code — CLAIM → EXTRACT → DOUBT → RECONCILE → STOP. Tự đặt câu hỏi về từng decision không tầm thường.  
**Dùng khi:** Stakes cao (production, security, irreversible), code unfamiliar, hoặc muốn second opinion.  
**Cách dùng:**
```
Hãy doubt-check approach này trước khi implement
```

#### `frontend-ui-engineering`
**Chức năng:** Component architecture, design systems, state management, responsive design, WCAG 2.1 AA accessibility. Workflow chuẩn cho UI.  
**Dùng khi:** Build hoặc sửa UI components, pages.  
**Cách dùng:** Tự kích hoạt khi đang build UI. Hoặc:
```
Build component X theo frontend-ui-engineering workflow
```

#### `api-and-interface-design`
**Chức năng:** Contract-first API design — Hyrum's Law, One-Version Rule, error semantics, boundary validation. Thiết kế API trước khi implement.  
**Dùng khi:** Thiết kế REST/GraphQL endpoint, module boundary, hoặc public interface.  
**Cách dùng:** Tự kích hoạt khi thiết kế API. Hoặc:
```
Design API cho feature X theo contract-first
```

---

### VERIFY — Prove it works

#### `browser-testing-with-devtools`
**Chức năng:** Test trên browser thật qua Chrome DevTools MCP — DOM inspection, console logs, network traces, performance profiling.  
**Dùng khi:** Build hoặc debug bất cứ thứ gì chạy trên browser.  
**Cách dùng:** Yêu cầu Chrome DevTools MCP được cài. Dùng khi:
```
Verify feature X trên browser với real runtime data
```

#### `debugging-and-error-recovery`
**Chức năng:** 5-step triage: Reproduce → Localize → Reduce → Fix → Guard. Stop-the-line rule, safe fallbacks. Không đoán mò.  
**Dùng khi:** Test fail, build break, behavior không như expect.  
**Cách dùng:** Tự kích hoạt khi gặp lỗi. Hoặc:
```
Debug lỗi X theo systematic approach
```

---

### REVIEW — Quality gates

#### `code-review-and-quality`
**Chức năng:** Review 5 chiều: correctness, readability, architecture, security, performance. Change sizing ~100 lines. Severity labels: Nit/Optional/FYI/Important/Critical.  
**Dùng khi:** Trước khi merge bất kỳ change nào.  
**Cách dùng:** Gõ `/review` hoặc:
```
Review code này theo 5-axis checklist
```

#### `code-simplification`
**Chức năng:** Chesterton's Fence (hiểu trước khi xóa), Rule of 500, reduce complexity — giữ nguyên exact behavior.  
**Dùng khi:** Code chạy đúng nhưng quá phức tạp, khó đọc, khó maintain.  
**Cách dùng:** Gõ `/code-simplify` hoặc:
```
Simplify file X, giữ nguyên behavior
```

#### `security-and-hardening`
**Chức năng:** OWASP Top 10 prevention, auth patterns, secrets management, dependency auditing. Three-tier boundary system.  
**Dùng khi:** Handle user input, auth, data storage, external integrations.  
**Cách dùng:** Tự kích hoạt khi code security-sensitive. Hoặc:
```
Security review feature X theo OWASP checklist
```

#### `performance-optimization`
**Chức năng:** Measure-first — Core Web Vitals targets, profiling workflows, bundle analysis, anti-pattern detection. Không optimize khi chưa đo.  
**Dùng khi:** Performance requirements tồn tại hoặc nghi ngờ regression.  
**Cách dùng:**
```
Profile và optimize X, target LCP < 2.5s
```

---

### SHIP — Deploy with confidence

#### `git-workflow-and-versioning`
**Chức năng:** Trunk-based development, atomic commits, change sizing ~100 lines, commit-as-save-point pattern.  
**Dùng khi:** Mọi code change (luôn luôn).  
**Cách dùng:** Tự áp dụng khi commit. Hoặc:
```
Tạo commit message chuẩn cho change này
```

#### `ci-cd-and-automation`
**Chức năng:** Shift Left, Faster is Safer, feature flags, quality gate pipelines, failure feedback loops.  
**Dùng khi:** Setup hoặc sửa CI/CD pipeline.  
**Cách dùng:**
```
Setup CI pipeline cho repo này theo best practices
```

#### `deprecation-and-migration`
**Chức năng:** Code-as-liability mindset, compulsory vs advisory deprecation, migration patterns, zombie code removal.  
**Dùng khi:** Remove old system/API, migrate users, sunset features.  
**Cách dùng:**
```
Plan deprecation cho API X với migration path
```

#### `documentation-and-adrs`
**Chức năng:** Architecture Decision Records (ADR), API docs, inline documentation standards — document the *why*, not the *what*.  
**Dùng khi:** Quyết định kiến trúc, thay đổi API, ship feature.  
**Cách dùng:**
```
Viết ADR cho decision chọn X thay vì Y
```

#### `shipping-and-launch`
**Chức năng:** Pre-launch checklists, feature flag lifecycle, staged rollouts, rollback procedures, monitoring setup.  
**Dùng khi:** Chuẩn bị deploy lên production.  
**Cách dùng:** Gõ `/ship` — sẽ chạy song song code-reviewer + security-auditor + test-engineer rồi tổng hợp GO/NO-GO.

---

## 3 Agent Personas

Specialist personas được `/ship` orchestrate song song. Cũng có thể gọi riêng.

### `code-reviewer`
**Vai trò:** Senior Staff Engineer  
**Chức năng:** Review code theo tiêu chuẩn "would a staff engineer approve this?" — 5-axis, severity labels.  
**Cách gọi:**
```
@code-reviewer review PR này
```

### `security-auditor`
**Vai trò:** Security Engineer  
**Chức năng:** Phát hiện vulnerabilities, threat modeling, OWASP assessment, secrets scanning.  
**Cách gọi:**
```
@security-auditor audit feature authentication này
```

### `test-engineer`
**Vai trò:** QA Specialist  
**Chức năng:** Test strategy, coverage analysis, Prove-It pattern — xác định test nào thiếu.  
**Cách gọi:**
```
@test-engineer check coverage gaps cho module X
```

---

## Workflow tham khảo

### Feature mới từ đầu
```
/spec → /plan → /build → /test → /review → /ship
```

### Bug fix nhanh
```
debugging-and-error-recovery → /test → /review
```

### Refactor
```
code-simplification → /test (ensure no regression) → /review
```

### API design
```
api-and-interface-design → spec-driven-development → /plan → /build
```

### Security audit
```
/review + @security-auditor song song → fix → /ship
```

---

## Skills tự động kích hoạt

Claude sẽ tự nhận diện context và dùng skill phù hợp mà không cần gọi explicit:

| Đang làm gì | Skill tự kích hoạt |
|-------------|-------------------|
| Thiết kế API/endpoint | `api-and-interface-design` |
| Build React component | `frontend-ui-engineering` |
| Gặp lỗi/exception | `debugging-and-error-recovery` |
| Code liên quan auth/input | `security-and-hardening` |
| Implement logic | `test-driven-development` |
| Commit/push code | `git-workflow-and-versioning` |

---

## Reference checklists

Được load on-demand khi skill cần:

| File | Nội dung |
|------|----------|
| `references/testing-patterns.md` | Test structure, naming, mocking, React/API/E2E examples |
| `references/security-checklist.md` | Pre-commit checks, auth, CORS, OWASP Top 10 |
| `references/performance-checklist.md` | Core Web Vitals targets, profiling commands |
| `references/accessibility-checklist.md` | Keyboard nav, screen readers, ARIA, WCAG |
