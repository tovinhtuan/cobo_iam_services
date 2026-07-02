-- 0110 down: no-op — deleted rows cannot be safely restored without knowing original intent.
-- Data is recoverable from backup or by re-running the original seed migrations
-- if a full rollback is needed.
SELECT 1;
