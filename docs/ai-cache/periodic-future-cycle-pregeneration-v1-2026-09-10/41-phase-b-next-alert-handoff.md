# Phase B handoff — Tenant "Cảnh báo tiếp theo"

## Phase A deliverable
Authoritative future `periodic_cycle` rows may exist before OpenAt (no disclosure_record).

## Phase B business goal
Tenant UI shows read-only "Cảnh báo tiếp theo" projected from future periodic_cycle.

## Must NOT require
- early disclosure_record
- early workflow snapshot
- early workflow tasks

## Candidate read-only fields
- type/template identity
- logical slot (cycle_label)
- T / cycle_start
- OpenAt
- DueAt
- applicability state (derived)

## Recommended data source
`future_periodic_cycle_projection` (query unmaterialized cycles where TodayHCM < OpenAt)

## Phase B implementation
Not in this phase. PHASE_B_NEXT_ALERT_IMPLEMENTED=false
