# B5 audit source trace
FILE=internal/workflowfulfillment/audit.go + service.go
SYMBOL=appendFulfillmentAudit / WithAudit
EVENTS=workflow.document_fulfillment.{upload,delete,replace}
TX_BOUNDARY=AFTER commit of mutation TX
FAILURE_POLICY=log error; do not rollback mutation
ACTOR=Subject (JWT user_id + membership_id + company_id)
OBJECT=fulfillment file id + metadata (record/instance/step/requirement)
B5_AUDIT_SOURCE_TRACE_COMPLETE=true
