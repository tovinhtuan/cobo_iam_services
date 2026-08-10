SELECT task_id, step_code, status FROM workflow_tasks WHERE workflow_instance_id=(SELECT workflow_instance_id FROM workflow_tasks WHERE task_id='019feb2d-db4b-7210-a8af-86991e4f5266');
