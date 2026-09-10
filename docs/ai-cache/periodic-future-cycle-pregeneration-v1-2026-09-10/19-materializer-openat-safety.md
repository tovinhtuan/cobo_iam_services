# Materializer OpenAt safety

CRITICAL: removed MATERIALIZATION_LOOKAHEAD=7
bufferDays=0; ListPendingCycles asOf=HCM today
Materializable only when TodayHCM >= COALESCE(open_at, cycle_start)
