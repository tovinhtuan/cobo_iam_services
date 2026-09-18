# Contract applied
GENERIC_EVIDENCE_OWNER=DISCLOSURE_WORKFLOW_STEP
TABLE=workflow_step_evidence_files
lifecycle ACTIVE|SUPERSEDED|DELETED; logical delete; append-only replace
max 20MiB; max 10 ACTIVE/step; namespace workflow-step-evidence
G2A+G2B foundation reused; not reimplemented
