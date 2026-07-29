# Phase 12.6A — Query log

Purposes and hashes only — no DSN, password, or legal text.

- seq=1 ts=2026-07-29T08:49:58Z purpose=q1/hash:8abfba7e type=SELECT rows=0 durMs=0 write=false
- seq=2 ts=2026-07-29T08:49:58Z purpose=select_version type=METADATA rows=1 durMs=0 write=false
- seq=3 ts=2026-07-29T08:49:58Z purpose=q2/hash:5d25dc17 type=SELECT rows=0 durMs=0 write=false
- seq=4 ts=2026-07-29T08:49:58Z purpose=session_txn_ro type=SELECT rows=1 durMs=0 write=false
- seq=5 ts=2026-07-29T08:49:58Z purpose=q3/hash:bbbc4ceb type=SELECT rows=0 durMs=0 write=false
- seq=6 ts=2026-07-29T08:49:58Z purpose=show_grants type=METADATA rows=0 durMs=0 write=false
- seq=7 ts=2026-07-29T08:49:58Z purpose=q4/hash:a6a253d9 type=SELECT rows=0 durMs=0 write=false
- seq=8 ts=2026-07-29T08:49:58Z purpose=set_session_txn_ro type=METADATA rows=0 durMs=0 write=false
- seq=9 ts=2026-07-29T08:49:58Z purpose=q5/hash:bce9e8b2 type=SELECT rows=0 durMs=0 write=false
- seq=10 ts=2026-07-29T08:49:58Z purpose=ro_probe_select1 type=SELECT rows=1 durMs=0 write=false
- seq=11 ts=2026-07-29T08:49:58Z purpose=q6/hash:45cd2bb9 type=SELECT rows=0 durMs=0 write=false
- seq=12 ts=2026-07-29T08:49:58Z purpose=ro_probe_txn_flag type=SELECT rows=1 durMs=0 write=false
- seq=13 ts=2026-07-29T08:49:58Z purpose=q7/hash:86f168ee type=SELECT rows=0 durMs=0 write=false
- seq=14 ts=2026-07-29T08:49:58Z purpose=set_session_txn_ro_rr type=METADATA rows=0 durMs=0 write=false
- seq=15 ts=2026-07-29T08:49:58Z purpose=q8/hash:45cd2bb9 type=SELECT rows=0 durMs=0 write=false
- seq=16 ts=2026-07-29T08:49:58Z purpose=txn_read_only_check type=SELECT rows=1 durMs=0 write=false
- seq=17 ts=2026-07-29T08:49:58Z purpose=q9/hash:d5249694 type=SELECT rows=0 durMs=0 write=false
- seq=18 ts=2026-07-29T08:49:58Z purpose=q10/hash:d5249694 type=SELECT rows=0 durMs=0 write=false
- seq=19 ts=2026-07-29T08:49:58Z purpose=schema_column_disclosure_type_versions_is_released type=METADATA rows=1 durMs=0 write=false
- seq=20 ts=2026-07-29T08:49:58Z purpose=q11/hash:d5249694 type=SELECT rows=0 durMs=0 write=false
- seq=21 ts=2026-07-29T08:49:58Z purpose=q12/hash:d5249694 type=SELECT rows=0 durMs=0 write=false
- seq=22 ts=2026-07-29T08:49:58Z purpose=schema_column_disclosure_type_versions_legal_basis type=METADATA rows=1 durMs=0 write=false
- seq=23 ts=2026-07-29T08:49:58Z purpose=q13/hash:d5249694 type=SELECT rows=0 durMs=0 write=false
- seq=24 ts=2026-07-29T08:49:58Z purpose=q14/hash:d5249694 type=SELECT rows=0 durMs=0 write=false
- seq=25 ts=2026-07-29T08:49:58Z purpose=schema_column_disclosure_type_versions_legal_bases_json type=METADATA rows=1 durMs=0 write=false
- seq=26 ts=2026-07-29T08:49:58Z purpose=q15/hash:db088778 type=SELECT rows=0 durMs=0 write=false
- seq=27 ts=2026-07-29T08:49:58Z purpose=count_total type=SELECT rows=1 durMs=0 write=false
- seq=28 ts=2026-07-29T08:49:58Z purpose=q16/hash:84479ade type=SELECT rows=0 durMs=0 write=false
- seq=29 ts=2026-07-29T08:49:58Z purpose=q17/hash:84479ade type=SELECT rows=0 durMs=0 write=false
- seq=30 ts=2026-07-29T08:49:58Z purpose=keyset_page type=SELECT rows=6 durMs=0 write=false
- seq=31 ts=2026-07-29T08:49:58Z purpose=q18/hash:90320d8b type=SELECT rows=0 durMs=0 write=false
- seq=32 ts=2026-07-29T08:49:58Z purpose=sql_group_approx type=SELECT rows=1 durMs=0 write=false

SELECT/METADATA count = 32
INSERT count = 0
UPDATE count = 0
DELETE count = 0
DDL count = 0
LOCK count = 0
