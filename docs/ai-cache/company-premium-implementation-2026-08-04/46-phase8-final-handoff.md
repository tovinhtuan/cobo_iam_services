# Phase 8 — Final handoff

**Verdict: `COMPANY_PREMIUM_DEV_READY`**

Company Premium đã được triển khai end-to-end đúng domain ownership và được xác minh trên DEV.

Backend sử dụng `company_subscriptions` làm commercial source-of-truth theo `company_id`, không lấy từ `user.subscriptionTier` và không dùng `CompanyTierResolver` làm billing source. Migration 0125, DEV seed, MySQL concurrency, shared Reader, additive API contract, STRICT error semantics, authorization và tenant isolation đã được kiểm tra bằng source, tests và runtime evidence.

Frontend đã loại bỏ Premium khỏi Personal Ops và hiển thị Premium trong Thông tin doanh nghiệp chỉ khi plan là PREMIUM + ACTIVE + COMPANY_SUBSCRIPTION. c_001 hiển thị Premium, c_002 `plan:null` không hiển thị; chuyển công ty không stale và không cần refresh.

Transient 503 trong switch flow đã được RCA là Nginx `api_per_ip` rate limit, không phải API/DB. Web proxy đã được điều chỉnh từ 5r/s burst 20 lên 20r/s burst 40, rate limiting vẫn được giữ. Sau fix, 10 vòng c_001↔c_002 đạt 0×5xx, 0 toast và 0 stale badge. Thay đổi chỉ recreate web; API, worker, MySQL và API binary không đổi.

Exact Phase 6 targeted set đã được reconcile thành 60 PASS với 0 task-related failures. Full suite vẫn có các lỗi pre-existing và không được tuyên bố PASS.

Verified-email positive state không có runtime fixture vì các tài khoản DEV đều `email_verified=false`; trạng thái ẩn là đúng và positive behavior được xác nhận bằng unit tests. AdminCenter personal-tier badge vẫn là `DEFERRED_PRODUCT_DECISION_ADMINCENTER_PERSONAL_TIER_BADGE`; task này không tuyên bố mọi user-level Premium UI đã bị loại bỏ.

Rollback cho FE, Nginx, Backend, DEV seed và migration đã được ghi rõ. Không Production, không credential leak, không user-tier fallback, không CompanyTierResolver billing use, không billing-sensitive exposure và không unrelated discard.

Phase 8 chỉ hoàn thiện evidence/handoff, không thay đổi runtime. Final verdict:

**COMPANY_PREMIUM_DEV_READY**
