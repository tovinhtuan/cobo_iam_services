# FE alert call chain
DeadlineDetail.loadDetail → reloadWorkflow(instanceId) Promise.all getById+listTasks (now allSettled fail-soft on tasks)
Identity: workflow_instance_id (correct). Steps API separate.
