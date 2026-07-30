# 01 — Source discovery

## FE CMS
`TemplateEditorScreen` → `WorkflowConfigurationPage` → `useWorkflowConfiguration` → `GET .../workflow/configuration`
Owns **global** workflow versioning only. Portal activate SoT remains `enterprise_workflow` (code comment).

## FE Alert
`DeadlineList` → `GET /api/v1/company/deadline-alerts` (current_step_name only)
`DeadlineDetail` → `GET .../deadlines/{id}/steps` (preferred) + instance + tasks

## BE Effective
`GetEffectiveWorkflow`: company_override > global_workflow > global_template (`ExtractTemplateWorkflow`)

## BE Materialize
`CreateAndSubmitRecordWithPlannedDate` → live effective → `MapEffectiveWorkflowToSnapshot` → `workflow_instances.snapshot_json` (immutable)

## BE Alert
List: snapshot meta for current step name. Steps API: frozen snapshot (+ step_states); empty snapshot falls back to live effective.
