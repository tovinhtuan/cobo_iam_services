# Static forbidden-logic audit

## Structure flags in deadline display path
Occurrences only in `resolvedDeadlineRuleDisplay.ts` as **type union + localization map keys** (not company profile reads).
`DisclosureDeadlineSection` / `DisclosureTypeDetail` deadline wiring: **no** `has_subsidiaries` / `has_subordinate_*` reads.

## Period-end wording in new formatter/section
`NONE` in `resolvedDeadlineRuleDisplay.ts` / `DisclosureDeadlineSection.tsx`.

## Due-date calculation helpers in disclosure-detail/
`NONE` (`addDays` / `workingDays` / `businessDays`).

## FE calculation of due date
Only `formatCalendarDateDdMmYyyy` regex on `YYYY-MM-DD`.
