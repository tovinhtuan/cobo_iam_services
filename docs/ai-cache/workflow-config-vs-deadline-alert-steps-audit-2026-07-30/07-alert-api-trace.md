# 07 — Alert API trace

| Endpoint | Status | Workflow fields |
|----------|--------|-----------------|
| GET /api/v1/company/deadline-alerts?q=... | 200 | current_step_name, workflow_instance_id, active_departments (no full steps) |
| GET /api/v1/company/deadlines/{rid}/steps | 200 | 4 steps from snapshot + runtime status |
| GET /api/v1/workflows/instances/{iid} | 200 | instance meta |
| GET /api/v1/workflows/instances/{iid}/tasks | **500** INTERNAL_ERROR | blocks detail UI |

See `network/api-trace-readonly.json`.
