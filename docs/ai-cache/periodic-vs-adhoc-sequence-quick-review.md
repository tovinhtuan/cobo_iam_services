# Quick Review — Sequence Periodic vs Ad-hoc (Current Code)

Ngày cập nhật: 2026-05-27  
Phạm vi: `cobo_iam_services` (runtime flow hiện tại)

## 1) Periodic flow (worker tick)

```mermaid
sequenceDiagram
  participant W as Worker Tick
  participant DS as disclosure.Service
  participant R as disclosure.Repository
  participant RC as RecordCreatorAdapter
  participant DR as disclosure_records
  participant PC as periodic_cycles

  W->>DS: SeedPeriodicCycles(now)
  DS->>R: ListActivePeriodicTypes + ListAllActiveCompanyIDs
  DS->>R: UpsertPeriodicCycle(type, company, cycle_label, due_date)
  R-->>DS: periodic_cycles seeded/updated

  W->>DS: MaterializePeriodicDisclosures(now, creator)
  DS->>R: ListPendingCycles(now, buffer=7 days)
  loop each pending cycle
    DS->>RC: CreateAndSubmitRecord(company, type, "m_system_worker", title, t0=now)
    RC->>DS: CreateRecord + SubmitRecord
    DS->>DR: insert/update disclosure record
    RC-->>DS: record_id (workflow_id currently empty in worker path)
    DS->>R: UpdateCycleRecord(cycle_id, record_id)
    R->>PC: set periodic_cycles.record_id
  end
```

Ghi chú hiện tại:
- Worker đang wire `RecordCreatorAdapter(disclosureSvc, nil, false)` nên nhánh periodic **chưa bật workflow instance/snapshot** trong worker.
- Cycle lỗi giữ trạng thái pending để retry ở tick sau.

## 2) Ad-hoc irregular flow (proposal -> approval -> alert)

```mermaid
sequenceDiagram
  participant U as User/Proposer
  participant AH as ad_hoc.Service
  participant AR as ad_hoc.Repository
  participant PC as Process Controller
  participant RC as RecordCreatorAdapter
  participant DS as disclosure.Service
  participant WF as workflow.Service
  participant DA as deadline_alerts.Service

  U->>AH: CreateProposal(type=irregular, process_controller, overrides)
  AH->>AR: Insert status=DRAFT

  U->>AH: SubmitProposal
  AH->>AR: DRAFT -> PENDING_FOCAL_APPROVAL (hoặc skip nếu auto-approve)

  PC->>AH: AdminApprove(proposal_id)
  AH->>AR: ReserveAdminApproval (idempotency)
  AH->>RC: CreateAndSubmitRecord(company, type, membership, title, final_t0?)
  RC->>DS: CreateRecord + SubmitRecord
  RC->>WF: CreateWorkflowInstanceInternal (workflowOn=true ở API server)
  RC-->>AH: record_id, workflow_instance_id
  AH->>AR: CompleteAdminApproval(status=APPROVED, record_id, workflow_instance_id)

  U->>DA: GET /company/deadline-alerts
  DA->>AR: ListRows(disclosure_records LEFT JOIN workflow_instances)
  DA-->>U: UPCOMING/DUE_SOON/OVERDUE hoặc PENDING_CONFIRM
  U->>DA: POST /deadline-alerts/{id}/confirm
  DA-->>U: status DONE
```

Ghi chú hiện tại:
- Ad-hoc chỉ cho template category `irregular`.
- Duyệt cuối bắt buộc đúng `process_controller_membership_id` đã chỉ định.
- Record terminal nhưng chưa confirm sẽ là `PENDING_CONFIRM`; chỉ thành `DONE` sau API confirm.

