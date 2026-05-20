-- 0036 down: no-op — data repair is intentionally irreversible.
-- The collation change and Mojibake fix cannot be safely rolled back
-- (we cannot know which rows were garbled before the fix).
-- To revert, restore from a pre-0036 database snapshot.

SET NAMES utf8mb4;
SELECT 1; -- no-op statement required by some migration runners
