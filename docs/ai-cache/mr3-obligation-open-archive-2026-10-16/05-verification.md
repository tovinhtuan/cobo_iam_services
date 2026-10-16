# 05: Kiểm chứng (chưa commit, chưa deploy)

Số liệu dưới đây là sau khi sửa các phát hiện review (Phase 6, `06-review.md`).

## BE `cobo_iam_services`
| Lệnh / test | Kết quả |
|---|---|
| MySQL 8 thật (`cobo_iam_mr1`): `ArchiveCycles` (10), `ObligationOpen` (2), `OpenTasksQuery` (1) | 13/13 pass |
| Unit mới | Đều pass: `TestTemplateArchived` và các test SQL contract (`deadlinealerts/app`), `TestListRows_keepsObligationsOfArchivedTemplates`, `cms_archive_cycles_test.go` (7, gồm `TestCmsArchiveImpactAndRestore_RequirePlatformCMSView`), `periodic_materialize_lease_test.go` (5 test MR-3: 409, 404 khi template lưu trữ, 404 khi template hoạt động, template vừa khôi phục, treo lỗi), `TestBuildImmediateActions_carriesTemplateArchived`, `TestGetOperationalOverview_myTaskCarriesTemplateArchived` |
| Kiểm ngược (tắt từng hành vi) | Cả 12 lần đều có test fail (chi tiết bên dưới). Khôi phục (file giống hệt bản lưu) thì pass |
| `go test ./...` so với HEAD `81924c1` | 0 fail mới (cùng tập fail có sẵn: 14 test, 5 package) |
| `go vet ./...` | Sạch (cảnh báo có sẵn ở `internal/workflowfulfillment/required_document_gate_test.go`) |
| `gofmt -l` | Sạch, trừ 6 file vốn đã không sạch ở HEAD (diff tối thiểu, không format lại cả file) |
| `docker compose -f docker-compose.dev.yml build api` | Built |
| Migration `0154` (chỉ đổi dữ liệu) trên MySQL 8 thật | Chạy 2 lần. Kỳ `PENDING/RETRY` của template đã lưu trữ chuyển `SUSPENDED`, vẫn giữ fence `materialized_at`; kỳ của template đang hoạt động giữ nguyên; `down` không làm gì. `TestMigration0154InDevRunner` pass |

Các lần tắt hành vi:
- M1: JOIN template lọc lại theo `active_version_no > 0`;
- M2: lưu trữ không treo kỳ;
- M3: khôi phục làm sống lại mọi kỳ (DL-05);
- M4: "kỳ sắp tới" hiện cả kỳ đã treo;
- M5: worker retry khi template đã lưu trữ;
- M6: cấu hình hạn chỉ lấy từ version đang hoạt động;
- M7: khôi phục không gom kỳ sót lại;
- M8: khôi phục bỏ cả kỳ chưa đến ngày mở;
- M9: khôi phục bỏ kỳ đã có hồ sơ;
- M10: worker không treo khi nhận 404;
- tắt kiểm tra `platform.cms.view` của quyền archive;
- bỏ điều kiện template `archived` của câu treo.

## FE `cobo_web_design`
| Lệnh / test | Kết quả |
|---|---|
| Test focused CMS (97 file) | 584/584 pass |
| `npx vitest run` toàn bộ so với HEAD | 0 fail mới (48 fail có sẵn ở cả hai, cùng tên); 3248 pass so với 3196 ở HEAD |
| `npm run lint` | 12 lỗi TS, giống hệt HEAD |
| `npm run build` | OK |
| `npm run check:mojibake` | OK |

## Chưa chạy
- Kiểm tra trên trình duyệt: đã làm trong smoke DEV sau deploy (`07-deploy-smoke.md`, 8/8), gồm trang chi tiết hồ sơ của template đã lưu trữ (`DeadlineDetail.tsx`: trang tải được, có nhãn). Loader của trang chưa có test cấp trang.
- Postman: collection chưa có request CMS template nào, nên không thêm. Contract đã ghi ở `docs/api-contracts-json.md`, mục "Open obligations of archived templates (MR-3, 2026-10-16)".

## Dữ liệu trước khi deploy (chỉ đọc)
- **DEV:** 312 hồ sơ QA của 6 template đã lưu trữ sẽ hiện lại kèm nhãn (đã chốt tại GATE A). Migration 0154 sẽ treo 4 kỳ `PENDING` của template đã lưu trữ.
- **Prod:**

```sql
-- Hồ sơ đang mở của template đã lưu trữ, theo template (sẽ hiện lại sau MR-3)
SELECT dt.type_id, COUNT(*) FROM disclosure_records dr JOIN disclosure_types dt ON dt.type_id = dr.type_id
WHERE (LOWER(dt.status) = 'archived' OR (dt.company_id IS NOT NULL AND LOWER(dt.review_status) = 'archived'))
  AND LOWER(TRIM(dr.status)) = 'draft' AND dr.submitted_at IS NULL GROUP BY dt.type_id;
-- Kỳ chưa sinh hồ sơ của template đã lưu trữ (migration 0154 sẽ chuyển các kỳ PENDING/RETRY sang SUSPENDED)
SELECT pc.type_id, pc.materialization_state, COUNT(*) FROM periodic_cycles pc JOIN disclosure_types dt ON dt.type_id = pc.type_id
WHERE dt.status = 'archived' AND pc.record_id IS NULL GROUP BY pc.type_id, pc.materialization_state;
-- Hồ sơ Draft không có template (type_id NULL hoặc không tồn tại): MR-3 bỏ INNER JOIN nên cũng hiện ra (review DB)
SELECT COUNT(*) FROM disclosure_records dr LEFT JOIN disclosure_types dt ON dt.type_id = dr.type_id
WHERE dt.type_id IS NULL AND LOWER(TRIM(dr.status)) = 'draft' AND dr.submitted_at IS NULL;
```
