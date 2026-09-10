# Materializer OpenAt source trace

MATERIALIZER_ENTRYPOINT=cmd/worker → MaterializePeriodicDisclosures → materializePeriodicDisclosures
MATERIALIZER_PENDING_QUERY=Repository.ListPendingCycles (mysql)
MATERIALIZER_OPENAT_PREDICATE (SQL list)= COALESCE(pc.open_at, pc.cycle_start, pc.due_date) <= asOf
MATERIALIZER_APP_GATE= skip if CycleStart.IsZero() ("periodic cycle missing cycle_start; skip materialize")
MATERIALIZER_TEST_FAKE= OpenAt else CycleStart (no due_date)
MATERIALIZER_COMMENT= bufferDays=0 require TodayHCM >= COALESCE(open_at, cycle_start)

EFFECTIVE business authority for successful materialization:
COALESCE(open_at, cycle_start)
because due_date-only rows cannot pass CycleStart requirement.
