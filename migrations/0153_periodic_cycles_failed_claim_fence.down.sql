-- Forward-only: clearing materialized_at again would let an old worker binary re-run FAILED
-- cycles and restart the Draft loop (PER-01).
SELECT 1;
