# Kế hoạch test (wf-bugfix GATE R / GATE V)

**Quy tắc bắt buộc**
1. Trong cùng một commit, viết test tái hiện **trước**, rồi mới sửa.
2. Chứng minh test **fail trên commit cha**: dán output test đỏ vào mô tả PR.
3. Sau khi sửa, test phải pass. Revert riêng bản sửa thì test phải fail lại.
4. Test `mysql-integration` cần `MYSQL_TEST_DSN` trỏ tới DB có chữ "test" trong tên và đã chạy migration. Nếu chưa có thì ghi `BLOCKED: no MYSQL_TEST_DSN`, và phải có thêm test unit hoặc source-guard thay thế.
5. Test "compile-fail" (dùng field mới) được tính là fail trên HEAD.

## 1. Hạ tầng test cần làm trước (Q-26)
- [ ] MySQL test local:
  - chạy `docker compose -f docker-compose.dev.yml up -d mysql`;
  - tạo DB `cobo_iam_test`;
  - chạy migration theo `migrations/run_dev_migrations.sh` (đã gồm 0152/0153);
  - export `MYSQL_TEST_DSN=…/cobo_iam_test?parseTime=true&loc=UTC`.
- [ ] Sửa các fake để mô phỏng đúng MySQL (PERF-47):
  - occurrence repo có 2 unique key (`occurrence_id`, `idempotency_key`) và hành vi `ON DUPLICATE KEY UPDATE`;
  - milestone scanner tôn trọng `limit`, thứ tự `scheduled_date, milestone_id` và cursor;
  - in-app repo có kiểm PK.
- [ ] Ghi baseline trên HEAD (để so sánh sau):
  - `go test ./... 2>&1 | grep -E '^(--- FAIL|FAIL)'` → kỳ vọng đúng các fail có sẵn: 6 test template-import (`disclosure/app`), `notification/app TestContract_VariableParity/workflow.approved` cùng các mục đã ghi ở `release-2026-10-10/06-release-verify.md`;
  - `go vet ./...` → chỉ còn `workflowfulfillment`.

## 2. Ma trận test tái hiện (GATE R)

### PR-1 Reminder (`02a`)
| ID | File | Test | Loại | HEAD |
|---|---|---|---|---|
| RMC-01 | `internal/reminder/app/workflow_step_eligibility_test.go` | `TestSeedOccurrences_LegacyStep2EligibleAfterStep1CompletedOnScreen`, `TestDecideWorkflowStepReminder` | unit | FAIL |
| RMC-01 | `internal/reminder/infra/mysql/…` | `TestMySQLWorkflowStepRuntimeReader` | integration | — |
| REM-06 | `internal/reminder/app/workflow_step_reminder_seed_test.go` | `TestSeedOccurrences_DeadMilestonesDoNotStarveBatch`, `TestSeedOccurrences_WaitRowsDoNotStarve` | unit (`-race`) | FAIL |
| REM-01 | `internal/reminder/app/…` | `TestSeedOccurrences_DuplicateMilestonesKeepSentOccurrence` | unit | FAIL |
| REM-01 | `internal/reminder/infra/mysql/occurrence_seed_guard_test.go` | `TestEnsureOccurrence_DoesNotOverwriteState` | source guard | FAIL |
| REM-01 | `internal/reminder/infra/mysql/…` | `TestMySQLEnsureOccurrence_DuplicateKeyReturnsExistingSent` | integration | — |
| REM-01 | `workflow/app/snapshot_test.go`; `disclosure/app/workflow_override_validation_test.go`; `disclosure/app/cms_workflow_facade_identity_test.go`; `adhoc/app/…` | `TestValidateSnapshot_RejectsDuplicateStepIdentity`, `TestValidateCompanyWorkflowOverrideSteps_RejectsDuplicateStepID`, `TestFacadeStepIdentities_GeneratedIDCollision`, `TestValidateFrozenProposalWorkflowForRuntime_DuplicateStepID` | unit | FAIL |
| REM-05 | `internal/reminder/app/dispatch_skip_defer_test.go` | `TestDispatchDue_SkippedOccurrenceIsDeferred`, `TestDispatchDue_SkippedOlderThan48hExpires` | unit | FAIL |
| RMC-02 | `internal/reminder/app/dispatch_window_test.go` | `TestDispatchDue_OutsideSendWindowSendsNothing`, `TestSeedOccurrences_LateBurstSeedsOnlyNearestOffset`, `TestDispatchDue_RechecksStepEligibility`, `TestPrepareDispatch_OverdueWording` | unit | FAIL |
| REM-03 | `internal/reminder/app/dispatch_outcome_test.go` | `TestDispatch_ReplayAfterCrashConvergesToSent` | unit | FAIL |
| REM-03 | `internal/reminder/infra/mysql/…` | `TestMySQLRecordDispatchOutcome_IdempotentAndGuarded` | integration | — |
| REM-04 | `internal/reminder/app/…` | `TestDispatchDue_EvaluatorErrorIsBounded` | unit | FAIL |
| RMC-06 | `internal/workflow/infra/mysql/milestones_status_test.go` | `TestReminderStatusFromRow` (+ `TestMySQLListByInstance_StatusFromOccurrence`) | unit (+ integration) | FAIL |
| RMC-03 | `internal/reminder/app/…` | `TestPrepareDispatch_WorkflowStepDefaultTemplate` | unit | FAIL |
| RMC-04 | `internal/reminder/app/…` | `TestDecideWorkflowStepReminder_DelayShiftWaits`, `TestDispatchDue_UsesShiftedStepEnd` | unit | FAIL |
| REM-10 | `internal/reminder/infra/inapp/bridge_test.go` | `TestBridge_SameOccurrenceTwiceCreatesOneRowPerUser` | unit (`-race`) | FAIL |
| RMC-09 | `internal/workflow/app/…`, `internal/reminder/app/…` | `TestMapEffectiveWorkflowToSnapshot_KeepsInstructions`, `TestPrepareDispatch_FallsBackToGlobalInstructionsWhenSnapshotEmpty` | unit | FAIL |

### PR-2 Tính hạn + periodic (`02b`)
| ID | File | Test | Loại | HEAD |
|---|---|---|---|---|
| PER-01 | `internal/disclosure/infra/mysql/repository_periodic_test.go` | `TestMarkPeriodicCycleFailedKeepsLegacyClaimFence` (+ integration với selector cũ) | source guard | FAIL |
| PERF-46 | `repository_periodic_test.go`; `periodic_oneshot/engine_test.go` | `TestReleasePeriodicCycleClaimRequeuesAsRetry`, `TestEngine_UsesDeterministicRecordID` | unit | FAIL |
| BES-38 | `internal/disclosure/app/periodic_applicable_to_seed_test.go` | `TestSeedPeriodicCycles_CompanyTypeOnlyForOwner` (+ SQL guard) | unit (compile-fail) | FAIL |
| PER-05 | `internal/workflow/app/service_test.go` | `TestCreateWorkflowInstance_MilestoneInsertFailureIsLogged` | unit | FAIL |
| PER-04 | `internal/disclosure/app/periodic_materialize_test.go` | `TestMaterializePeriodic_CancelAfterClaimStillFinalizes`, `TestMaterializePeriodic_ReapsStaleClaims` | unit (`-race`) | FAIL |
| PER-03 | `periodic_materialize_test.go` | `TestMaterializePeriodic_PermanentGateErrorMarksFailed`, `TestMaterializePeriodic_RetryableGateKeepsRetry`, `TestMarkPeriodicCycleRetryCapsAttempts` | unit | FAIL |
| DUE-01 | `internal/holiday/infra/mysql/repository_test.go` | `TestReplaceCalendar_BindsCivilDate` (+ `TestReplaceCalendar_RoundTrip_MySQL`) | unit (+ integration) | FAIL |
| DUE-03 | `internal/disclosure/app/holiday_calendar_files_test.go` | `TestNonTradingDayFiles_RequiredYears`, `_FixedSolarHolidays`, `_NextYearFromOctober`, `_DeployArtifactsInSync` | unit (đọc file) | FAIL |
| PER-06 | `internal/disclosure/app/periodic_daytype_test.go` | `TestSeedPeriodicCycles_MissingHolidayYearIsLogged` | unit | FAIL |
| DLA-01 | `deadlinealerts/app/status_test.go`; `steps_algorithm_test.go`; `personalops/app/mine_test.go` | `TestRemainingDaysFromDue_UTCInstantEarlyHCM`, `TestComputeDeadlineSteps_TodayFromUTCInstant`, `TestRemainingDays_UTCInstantEarlyHCM` | unit | FAIL |
| DLA-05 | `deadlinealerts/app/service_test.go` | `TestListDeadlineAlerts_OneUnresolvableRowDoesNotFailList` (+ `…AnchoredToCreatedAt` sau Q-10) | unit | FAIL |
| DUE-02 | `periodic_daytype_test.go`; `deadlinealerts/app/service_test.go` | `TestSeedPeriodicCycles_DayTypeResolution`, `TestPeriodicDayType_PreviewMatchesWorker`, `TestListDeadlineAlerts_NullPlannedDateUsesCalendarDefault` | unit | FAIL |
| DUE-09 | `periodic_future_pregen_test.go` | `TestFuturePregen_OpenDaysBeforeTExceedsLead` | unit | FAIL |
| DUE-07 | `disclosure/infra/workflow/bootstrap_test.go` | `TestEnsureOnSubmit_T0IsSubmitDateNotPlannedDate` | unit | FAIL |
| DUE-08 | `timeline_test.go` | `TestStepTimelines_ExceedsDueFlagged` | unit | FAIL |
| Q-05 (DUE-10 đã đóng) | `periodic_daytype_test.go`; `deadline_calculator_test.go` | `TestSeedPeriodicCycles_CalendarDueOnWeekendIsKept`, `TestFixedDate_DefaultKeepsDateOnNonTradingDay`, `TestFixedDate_MoveNextWorkingOnlyWhenTemplateSaysSo` | unit, test khoá quyết định | PASS (hành vi hiện tại đã đúng; test chỉ để giữ nguyên) |
| DUE-02 (theo Q-05) | `deadline_calculator_test.go`; `first_occurrence_preview_test.go` | `TestPreview_UsesRulesDayTypeWhenConfigEmpty` (rules=calendar, cfg trống → ngày lịch); `TestTemplateValidation_RequiresDayTypeWhenDeadlineDays` | unit | FAIL |

### PR-3 Ad-hoc (`02c`)
| ID | File | Test | Loại | HEAD |
|---|---|---|---|---|
| AHA-01 | `deadlinealerts/infra/mysql/list_rows_membership_test.go` | `TestListRowsMembership_IncludesApprovedAdHocBranch` + mở rộng acceptance matrix (+ `TestIntegration_ListRows_ApprovedAdHocIncluded`) | unit (+ integration) | FAIL |
| AHA-07 | `adhoc/infra/disclosure/record_creator_bootstrap_test.go`; `adhoc/app/…` | `TestAdHocFinalize_DoesNotInvokeSubmitBootstrap`, `TestBuildCreateRecordOptsForFinalize_ResumesWorkflowOnDuplicate`, `TestReplay_OnlyTemplateInstance_CreatesProposalInstance` (+ `TestIntegration_InstanceSelection_PrefersProposalSnapshot`) | unit (+ integration) | FAIL |
| AHA-02 | `workflow/app/multistep_runtime_test.go` | `TestV2MultiStep_FinalConfirm_RunsCompletionSideEffects`, `TestLegacy_FinalReview_RunsCompletionSideEffects`, `TestFinalReject_NoApprovalSideEffects` | unit | FAIL |
| AHA-03 | `adhoc/app/…`; `proposal_due_test.go` | `TestApprove_MultiReviewer_FinalDatesFromAllVotes`, `TestFormatProposalDueDate_FinalT0Overrides` (+ `TestIntegration_ReserveVote_RetryKeepsAdjustments`) | unit (+ integration) | FAIL |
| AHA-04 | `adhoc/infra/notification/notifier_test.go` | `TestShortNote_VietnameseKeepsValidUTF8` | unit | FAIL |
| AHA-05 | `adhoc/app/…` | `TestSubmitProposal_InactiveReviewer_Rejected`, `TestSubmitProposal_CreatorLacksDisclosureCreate_Rejected`, `TestFinalize_V3InactiveAssignee_FailsBeforeRecordCreate` | unit | FAIL |
| AHA-06 | `adhoc/app/…` | `TestCancelDuringFinalize_AbandonsMaterialization`, `TestRejectAfterFailedFinalize_AbandonsMaterialization` | unit (`-race`) | FAIL |
| AHA-09a | `workflow/infra/notification/lookup_integration_test.go` | `TestIntegration_LookupCreatorEmail_MembershipCreatedBy`, `_InactiveMembershipNoEmail` (+ source guard) | integration | — |
| AHA-12 | `adhoc/app/…` | `TestPatchDraft_T0Only_KeepsAbsoluteDeadline` | unit | FAIL |
| AHA-13 | — | query kiểm tra trước ở `04` §6 | dữ liệu | — |

### PR-4 / PR-5 (`02d`)
| ID | File | Test | Loại | HEAD |
|---|---|---|---|---|
| DLA-02 | `deadlinealerts/infra/mysql/enrich_helpers_test.go`; `list_rows_draft_enrich_integration_test.go` | `TestEnrichHelpers_doNotExcludeDraft`, `TestIntegration_ListRowsPage_DraftTaskAssigneeEnriched` | source guard + integration | FAIL |
| DLA-03 | `portaldashboard/app/deadline_snapshot_test.go` | `TestFetchDeadlines_7dCountsFutureUpcomingAndOldOverdue` | unit | FAIL |
| DLA-04 | `portaldashboard/app/aggregate_test.go`; `completion_test.go` | `TestSeverityFromOnTimeRate_highRateIsGood`, `TestComputeCompletion_confirmedDraftCounts` | unit (compile-fail) | FAIL |
| DLA-07 | `list_rows_membership_test.go` | `TestListRows_orderByHasUniqueTieBreaker` (+ integration 14 dòng) | source guard | FAIL |
| DLA-09 | `disclosure/infra/observe/…` | test collector | unit (compile-fail) | FAIL |
| DLA-11 | `deadlinealerts/infra/mysql/…` | `TestConfirmDeadlineAlert_firstWriterWins` (+ `TestIntegration_ConfirmTwice_KeepsFirst`) | source guard | FAIL |
| DLA-12 | WEB `src/services/deadlineAlertsApi.test.ts`; `deadlineAlertUrgencyPresentation.test.ts` | "unknown status does not map to Upcoming"; nhãn "Đến hạn hôm nay" | Vitest | FAIL |
| RMC-05 | `reminder/infra/mysql/company_status_filter_test.go`; test mysql của disclosure | `TestListDispatchCandidates_excludesInactiveCompany`, `TestListAllActiveCompanyIDs_includesSuspended` | source guard (+ integration) | FAIL |
| RMC-07 | `reminder/app/notification_rules_dispatch_test.go` | `TestDispatchDue_FlagOn_InAppOnly_NoEmailOneInApp`, `TestDispatchDue_FlagOn_InAppDisabled_NoInApp`, `TestNotificationRulesEvaluator_NoRowAppliesDefaults` | unit | FAIL |
| RMC-08 | `reminder/app/notification_rules_test.go`; test policy | `TestNotificationRulesEvaluator_WorkflowOffset1_NotGatedByCompanySchedules`, `TestApplyRecipientPolicies_DepartmentFocalResolvesCompanyDepartment` | unit | FAIL |

## 3. Kịch bản bắt buộc cho worker (theo `backend-worker-idempotent`)
| Kịch bản | Test đại diện |
|---|---|
| Chạy lặp nhiều tick | REM-01 (3 tick), REM-06 (2 tick), REM-05 (2 tick) |
| Crash sau side effect | REM-03 `ReplayAfterCrashConvergesToSent`; PER-04 `CancelAfterClaimStillFinalizes` |
| 2 worker đồng thời | REM-01 + REM-02 (2 service dùng chung repo, `-race`); PER-03/04 claim một bên thắng (đã có test của `cabbecf`) |
| Gửi muộn hoặc ngoài khung giờ | RMC-02 |
| Thiếu dữ liệu / lỗi phụ thuộc | REM-04 (evaluator lỗi); DLA-05 (dòng không tính được hạn); PER-06 (thiếu năm) |

## 4. Lệnh verify (GATE V) theo PR
```bash
# PR-1
go test ./internal/reminder/... ./internal/workflow/... ./internal/inappnotification/... ./internal/adhoc/app/ ./internal/deadlinealerts/... -count=1
go test -race ./internal/reminder/... ./internal/inappnotification/... -count=1
go test ./internal/disclosure/app/ ./internal/notification/app/ -count=1
# PR-2
go test ./internal/holiday/... ./internal/deadlinealerts/... ./internal/personalops/... ./internal/disclosure/... ./internal/workflow/... ./internal/portaldashboard/... -count=1
go test -race ./internal/disclosure/app ./internal/workflow/app -count=1
# PR-3
go test ./internal/adhoc/... ./internal/deadlinealerts/... ./internal/workflow/... ./internal/disclosure/... ./internal/personalops/... ./internal/portaldashboard/...
go test -race ./internal/adhoc/app/ ./internal/workflow/app/
# PR-4/5
go test ./internal/deadlinealerts/... ./internal/portaldashboard/... ./internal/reminder/... ./internal/companyaccess/... -count=1
# chung
go vet ./... && gofmt -l ./internal ./cmd
MYSQL_TEST_DSN=... go test -run 'MySQL|Integration' ./internal/... -count=1
docker compose -f docker-compose.dev.yml build api worker
# WEB (PR-4)
npx vitest run src/services src/pages/portal/deadlines src/features/portal-dashboard && npm run lint && npm run build
```
Bỏ qua bước nào thì ghi `BLOCKED: <lý do>`.

## 5. Smoke / SQL trên DEV sau deploy

Chạy bằng `qa_ro` (read-only). Mọi thao tác ghi dữ liệu phải được cho phép trước.

**Sau PR-1a**
- Milestone chết:
  ```sql
  SELECT COUNT(*) FROM workflow_step_milestones
  WHERE reminder_sent=0 AND milestone_type LIKE 'due_minus_%' AND scheduled_date<UTC_DATE();
  ```
  Kỳ vọng < ~40 (trước deploy 136+).
- Không có occurrence gửi lặp:
  ```sql
  SELECT occurrence_id FROM reminder_delivery_attempts
  GROUP BY 1 HAVING SUM(status='SENT')>1 OR COUNT(*)>5;
  ```
  Kỳ vọng 0.
- Không có milestone trùng:
  ```sql
  SELECT workflow_instance_id, step_id, milestone_type, COUNT(*) FROM workflow_step_milestones
  WHERE milestone_type LIKE 'due_minus_%' GROUP BY 1,2,3 HAVING COUNT(*)>1;
  ```
  Kỳ vọng 0.
- Theo `step_order`: bước ≥2 phải có seed sau khi có bước được hoàn tất trên màn hình.
- API: `GET /api/v1/workflows/instances/{id}/reminders` hiện `cancelled` cho milestone đã obsolete.

**Sau PR-1b**
- Giờ gửi chỉ trong 7..20 (giờ VN):
  ```sql
  SELECT HOUR(CONVERT_TZ(last_attempt_at,'+00:00','+07:00')) h, COUNT(*)
  FROM reminder_occurrences WHERE last_attempt_at>UTC_TIMESTAMP()-INTERVAL 1 DAY GROUP BY h;
  ```
- Không có dòng PENDING cũ hơn 48h. Không có dòng `DISPATCHING` cũ hơn 5 phút.

**Sau PR-2a**
- `/metrics`: `cobo_holiday_calendar_year_available` = 1 cho cả 2 năm.
- Log worker không có `holiday_year_missing`.
- `SELECT materialization_state, COUNT(*) FROM periodic_cycles GROUP BY 1;` ổn định; CLAIMED cũ = 0.

**Sau PR-3**
- Proposal v3 mới chỉ có 1 instance `proposal_snapshot_v3`.
- `/deadline-alerts` có record ad-hoc đã duyệt với hạn đúng.
- Confirm ở bước cuối → record Approved + email `workflow.approved`.

**Sau PR-4**
- Danh sách ENT: cột phòng ban/bước có dữ liệu; duyệt `page_size=5` không trùng dòng.
- Dashboard 7d và 30d: KPI mở bằng nhau.
- Severity đúng hạn không còn bị đảo.

**Sau PR-5**
- Simulate `in_app` / offset workflow 1 → would-send.
- Reminder của company inactive không tăng qua 3 tick.
