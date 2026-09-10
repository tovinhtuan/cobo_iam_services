# Eligibility
Future cycle exists AND record_id IS NULL AND active template AND company scope AND TodayHCM < COALESCE(open_at,cycle_start,due_date)
