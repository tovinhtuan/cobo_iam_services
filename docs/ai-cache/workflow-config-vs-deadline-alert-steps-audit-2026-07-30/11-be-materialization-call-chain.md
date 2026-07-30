# 11 — BE materialization call chain

periodic_oneshot Apply → ClaimCycle → CreateAndSubmitRecordWithPlannedDate
→ GetEffectiveWorkflow (global_template) → MapEffectiveWorkflowToSnapshot
→ workflow_instances.snapshot_json + first workflow_tasks row (assignee m_system_oneshot).
Record does **not** update when CMS global config later changes (immutable snapshot).
