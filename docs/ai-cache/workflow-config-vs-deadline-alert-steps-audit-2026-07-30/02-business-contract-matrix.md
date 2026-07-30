# 02 — Business contract matrix

| Layer | Source | Mutability | Must match live CMS Workflow tab? |
|-------|--------|------------|-----------------------------------|
| A CMS Config | `global_workflow_versions` | Draft/publish/activate | Self |
| B Active global | `global_workflows.active_version_no` | Via activate | Yes for global path |
| C Effective | override → global → template | Live resolve | **No** (different SoT) |
| D Snapshot | `workflow_instances.snapshot_json` | Immutable after materialize | No |
| E Runtime tasks | `workflow_tasks` + step_states | Runtime | No |
| F Alert API | Snapshot / current pointer | Read | No |
| G Alert UI | F + steps API | Read | No |

Contract: OQ-WF-05 / WF5-A — periodic materialize uses **effective** workflow.
OPEN_CONTRACT_DECISION: Should CMS Workflow tab always mirror `enterprise_workflow` / effective for operators?
