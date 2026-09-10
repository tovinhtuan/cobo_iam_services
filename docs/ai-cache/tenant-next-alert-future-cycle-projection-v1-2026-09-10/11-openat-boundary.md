# OpenAt
COALESCE(open_at, cycle_start, due_date) matches materializer ListPendingCycles
Today < OpenAt → NEXT; Today >= OpenAt → NOT_NEXT (even if record lag)
