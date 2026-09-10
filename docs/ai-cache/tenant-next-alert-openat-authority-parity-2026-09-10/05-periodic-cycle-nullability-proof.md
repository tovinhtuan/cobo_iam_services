# Nullability

PERIODIC_CYCLE_CYCLE_START_DB_NULLABLE=true (migration 0080 DATE NULL)
PERIODIC_CYCLE_CYCLE_START_MODEL_NULLABLE=true (time.Time zero)
PERIODIC_CYCLE_CAN_EXIST_WITH_NULL_CYCLE_START=true (Upsert allows nil when CycleStart.IsZero; legacy pre-0080)
open_at NULLABLE (0131)
due_date NOT NULL (0039)
