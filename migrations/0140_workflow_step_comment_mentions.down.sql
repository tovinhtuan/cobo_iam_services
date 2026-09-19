-- 0140 down: DEV/test or operator-confirmed only.
-- PRODUCTION_ROLLBACK=forward_fix_or_flag_off
-- DEV_DOWN=DROP_TABLE_ALLOWED_WITH_CONFIRMATION
-- Dropping this table destroys mention rows; do not use as production rollback.

SET NAMES utf8mb4;

DROP TABLE IF EXISTS workflow_step_comment_mentions;
