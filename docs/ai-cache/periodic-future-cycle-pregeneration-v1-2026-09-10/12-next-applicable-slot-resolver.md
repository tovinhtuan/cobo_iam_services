# ResolveNextApplicableLogicalSlot

File: internal/disclosure/app/next_applicable_slot.go

- Start NextLogicalSlot(current)
- SPECIFIC_SLOT AF: jump to boundary when next < boundary and boundary > current
- Forward walk only; bounded iterations by frequency
- ONE slot only; HISTORICAL_BACKFILL=false
