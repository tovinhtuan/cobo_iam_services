# QA test matrix CSV summary

- File tao moi: `docs/qa-test-matrix.csv`.
- Dinh dang: `Endpoint | Case | Input | Expected status | Expected error code`.
- Muc dich: import truc tiep vao test tracker.
- Bao phu:
  - Health + readiness
  - Auth + me + internal authorize
  - Disclosure (bao gom idempotency replay/conflict)
  - Workflow + notification
  - Admin access APIs

Luu y khi import:
- CSV da dung header dong 1.
- Cac field JSON duoc escape bang `""` de tuong thich parser CSV.
