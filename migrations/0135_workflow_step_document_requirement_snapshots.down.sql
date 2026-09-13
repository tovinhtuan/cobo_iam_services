-- Rollback B1 requirement snapshots. Safe: no fulfillment FKs; empty or additive-only data.
DROP TABLE IF EXISTS workflow_step_document_requirement_snapshots;
