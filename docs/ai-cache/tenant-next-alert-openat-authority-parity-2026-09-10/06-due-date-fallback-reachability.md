# Due date fallback

DUE_DATE_FALLBACK_REACHABLE=true in ListPendingCycles SQL when open_at and cycle_start both NULL.
But materialize skips those rows → due_date cannot successfully open runtime.
Next Alert MUST NOT use due_date (would invent UPCOMING window for non-materializable rows / confuse boundary).
