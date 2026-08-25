# 05 — Projected cycle writes

```text
CYCLE_CARDINALITY_MODEL=one row per (company_id × type_id × cycle_label)
CYCLE_LOGICAL_UNIQUE_CONSTRAINT=UNIQUE uq_pc_type_company_label (type_id, company_id, cycle_label)
CYCLE_CREATION_WRITES=cycle_id, type_id, company_id, cycle_label, cycle_start, open_at, due_date
EXISTING_CYCLE_UPSERT_BEHAVIOR=NOOP (ON DUPLICATE KEY UPDATE cycle_id=cycle_id; sticky snapshot)
EXISTING_OCCURRENCES_MUTATED_BY_SEED=false
WILL_ENABLING_SEED_BACKFILL_OLD_SLOTS=false
```

Projection (HCM 2026-08-25, strict applicability):

```text
EXISTING_CURRENT_CYCLES=0  # among projected eligible combos
PROJECTED_NEW_CYCLES_ON_FIRST_SEED_RUN=52
AFFECTED_COMPANIES=4
BY_FREQUENCY new cycles: daily=4 monthly=16 quarterly=12 yearly=20
```

Target row:

```text
Template: Bảng tính lương nhân viên ngày
Root: bang-tinh-luong-nhan-vien-thang-ban-sao
Current slot: 2026-08-25
AF: 2026-08-25 eligible
Current cycle: absent
Projected first seed: CREATE × 4 companies
```

Note: worker `seeded` return count increments on every successful Upsert including NOOP — **do not** use log `seeded` as “new cycles”; use DB delta.
