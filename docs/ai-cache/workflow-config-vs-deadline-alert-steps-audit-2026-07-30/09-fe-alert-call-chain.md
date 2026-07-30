# 09 — FE alert call chain

DeadlineList → deadline-alerts list → card uses `current_step_name` only.
DeadlineDetail → deadlines/{id}/steps (preferred timeline) + instance + **tasks** (hard fail on 500).
effective-workflow loaded for labels; not merged into primary timeline.
