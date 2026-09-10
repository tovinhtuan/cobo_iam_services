# Next Alert OpenAt source trace

NEXT_ALERT_ENTRYPOINT=GET /api/v1/company/deadline-alerts/next
NEXT_ALERT_QUERY=ListNextAlertCycles
BEFORE= COALESCE(open_at, cycle_start, due_date)
AFTER= COALESCE(open_at, cycle_start)
Constant: MaterializerEffectiveOpenAtSQL
