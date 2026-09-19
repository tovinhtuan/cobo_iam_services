-- 0139 down: DEV/test or operator-confirmed only.
-- Production: use forward-fix; dropping table destroys comment history.

SET NAMES utf8mb4;

DROP TABLE IF EXISTS workflow_step_comments;
