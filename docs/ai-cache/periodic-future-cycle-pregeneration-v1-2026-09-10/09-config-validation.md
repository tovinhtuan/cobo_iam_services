# Validation

BE: ValidatePeriodicCycleGenerationLeadDays — nil OK; else 0..90
FE: Number.isInteger + range gate onChange
Tests: absent/0/1/20/90 valid; -1/91/decimal blocked at UI; BE rejects out of range on write
