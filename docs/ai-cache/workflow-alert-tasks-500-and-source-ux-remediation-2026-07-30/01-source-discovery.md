# Source discovery
FE: DeadlineDetail → workflowInstancesApi.listTasks → GET /api/v1/workflows/instances/{id}/tasks
BE: handler.listInstanceTasks → ListInstanceTasks → ListTasksByInstance (LEFT JOIN users + department subquery)
CMS: WorkflowConfigurationPage + WorkflowDualSourcePanel + getEffectiveWorkflow
