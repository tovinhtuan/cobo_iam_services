-- Rollback note:
-- Do NOT restore milestone_type ENUM after live due_minus_Nd values exist.
-- Reverting to ENUM would truncate those values to '' (non-strict) or fail (strict).
-- Operator rollback: stop worker, inventory DISTINCT milestone_type, keep VARCHAR.
-- Destructive ENUM restore is intentionally omitted.

SELECT 1;
