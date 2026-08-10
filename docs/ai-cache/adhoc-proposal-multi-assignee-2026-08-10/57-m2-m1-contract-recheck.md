# M2 — M0/M1 contract recheck
- M0: ANY_ASSIGNEE_COMPLETES_STEP, ONE_LOGICAL_TASK_MANY_ASSIGNEES, schema_version=3, workflow_task_assignees authority
- M1: assignee_membership_ids write authority; submit-time head; v3 runtime was gated
- Drift: none → proceed M2 runtime
- Marker: M0/M1 rechecked OK
