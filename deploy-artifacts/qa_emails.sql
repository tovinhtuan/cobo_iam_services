-- Real-send QA: enqueue the 5 P0-fixed templates to the DEV QA inbox.
-- Worker (embed templates, real Gmail SMTP) consumes outbox event_type='email.dispatch'.
SET NAMES utf8mb4;
SET @to := 'tvttthptlvh@gmail.com';
SET @portal := 'http://88.216.208.0:3000';

-- 1) auth.user_invitation.existing_user --------------------------------------
SET @n1 := UUID();
INSERT INTO email_notifications
  (email_notification_id, recipient_email, template_key, locale, status,
   idempotency_key, variables_json_sanitized, created_at, updated_at)
VALUES
  (@n1, @to, 'auth.user_invitation.existing_user', 'vi', 'pending',
   CONCAT('qa-', @n1), '{}', NOW(), NOW());
INSERT INTO outbox_events
  (event_id, aggregate_type, aggregate_id, event_type, payload_json, status, available_at)
VALUES
  (UUID(), 'email_notification', @n1, 'email.dispatch',
   JSON_OBJECT('notification_id', @n1, 'variables', JSON_OBJECT(
     'display_name', N'Anh/Chị Thành',
     'company_name', N'Công ty CP Chứng khoán CoBo',
     'portal_url', @portal)),
   'pending', NOW());

-- 2) adhoc.controller_review_requested ---------------------------------------
SET @n2 := UUID();
INSERT INTO email_notifications
  (email_notification_id, recipient_email, template_key, locale, status,
   idempotency_key, variables_json_sanitized, created_at, updated_at)
VALUES
  (@n2, @to, 'adhoc.controller_review_requested', 'vi', 'pending',
   CONCAT('qa-', @n2), '{}', NOW(), NOW());
INSERT INTO outbox_events
  (event_id, aggregate_type, aggregate_id, event_type, payload_json, status, available_at)
VALUES
  (UUID(), 'email_notification', @n2, 'email.dispatch',
   JSON_OBJECT('notification_id', @n2, 'variables', JSON_OBJECT(
     'proposal_id', 'prop-qa-controller-001',
     'proposal_title', N'Công bố thông tin về việc thay đổi nhân sự cấp cao',
     'proposal_content', N'Bổ nhiệm ông Nguyễn Văn A giữ chức Tổng Giám đốc kể từ ngày 15/06/2026.',
     'company_name', N'Công ty CP Chứng khoán CoBo',
     'creator_name', N'Trần Thị B (Focal)',
     'portal_url', @portal)),
   'pending', NOW());

-- 3) adhoc.proposal_approved -------------------------------------------------
SET @n3 := UUID();
INSERT INTO email_notifications
  (email_notification_id, recipient_email, template_key, locale, status,
   idempotency_key, variables_json_sanitized, created_at, updated_at)
VALUES
  (@n3, @to, 'adhoc.proposal_approved', 'vi', 'pending',
   CONCAT('qa-', @n3), '{}', NOW(), NOW());
INSERT INTO outbox_events
  (event_id, aggregate_type, aggregate_id, event_type, payload_json, status, available_at)
VALUES
  (UUID(), 'email_notification', @n3, 'email.dispatch',
   JSON_OBJECT('notification_id', @n3, 'variables', JSON_OBJECT(
     'proposal_title', N'Công bố thông tin về việc thay đổi nhân sự cấp cao',
     'proposal_content', N'Bổ nhiệm ông Nguyễn Văn A giữ chức Tổng Giám đốc kể từ ngày 15/06/2026.',
     'company_name', N'Công ty CP Chứng khoán CoBo',
     'record_id', 'rec-qa-approved-001',
     'portal_url', @portal)),
   'pending', NOW());

-- 4) adhoc.proposal_rejected -------------------------------------------------
SET @n4 := UUID();
INSERT INTO email_notifications
  (email_notification_id, recipient_email, template_key, locale, status,
   idempotency_key, variables_json_sanitized, created_at, updated_at)
VALUES
  (@n4, @to, 'adhoc.proposal_rejected', 'vi', 'pending',
   CONCAT('qa-', @n4), '{}', NOW(), NOW());
INSERT INTO outbox_events
  (event_id, aggregate_type, aggregate_id, event_type, payload_json, status, available_at)
VALUES
  (UUID(), 'email_notification', @n4, 'email.dispatch',
   JSON_OBJECT('notification_id', @n4, 'variables', JSON_OBJECT(
     'proposal_id', 'prop-qa-rejected-001',
     'proposal_title', N'Công bố thông tin về kết quả kinh doanh quý II/2026',
     'proposal_content', N'Doanh thu quý II đạt 1.250 tỷ đồng, tăng 18% so với cùng kỳ.',
     'company_name', N'Công ty CP Chứng khoán CoBo',
     'reject_reason', N'Thiếu tài liệu đính kèm theo Thông tư 96/2020/TT-BTC.',
     'portal_url', @portal)),
   'pending', NOW());

-- 5) reminder.disclosure_deadline --------------------------------------------
SET @n5 := UUID();
INSERT INTO email_notifications
  (email_notification_id, recipient_email, template_key, locale, status,
   idempotency_key, variables_json_sanitized, created_at, updated_at)
VALUES
  (@n5, @to, 'reminder.disclosure_deadline', 'vi', 'pending',
   CONCAT('qa-', @n5), '{}', NOW(), NOW());
INSERT INTO outbox_events
  (event_id, aggregate_type, aggregate_id, event_type, payload_json, status, available_at)
VALUES
  (UUID(), 'email_notification', @n5, 'email.dispatch',
   JSON_OBJECT('notification_id', @n5, 'variables', JSON_OBJECT(
     'disclosure_title', N'Báo cáo tài chính bán niên 2026',
     'due_date', '20/06/2026',
     'company_name', N'Công ty CP Chứng khoán CoBo',
     'portal_url', @portal)),
   'pending', NOW());

SELECT email_notification_id, template_key, recipient_email, status
FROM email_notifications WHERE recipient_email = @to ORDER BY created_at DESC LIMIT 5;
