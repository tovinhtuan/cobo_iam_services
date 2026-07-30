# 12 — BE alert call chain

ListDeadlineAlerts → disclosure_records + workflow_instances snapshot meta → current_step_name.
ListDeadlineSteps → loadWorkflowForRecord prefers snapshot_json; maps order/status from snapshot + step_states.
