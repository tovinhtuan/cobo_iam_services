# Email Program — DEV Deploy + Real Email QA (2026-06-10)

- **task type:** implement → deploy → runtime QA (Phase D/E/F of Email Program closure)
- **objective:** Bring Email Program to READY FOR RELEASE: deploy P0 fixes to DEV, send real emails to `tvttthptlvh@gmail.com`, verify content + CTA end-to-end.

## Implemented / discovered
- DEV deploy mechanism: `docker-compose.artifacts.yml` on `root@88.216.208.0:21239` path `/root/cobo_project`. Pre-built binaries `bin/{api,worker}` (alpine runtime), FE `web/dist` via nginx. `EMAIL_TEMPLATE_SOURCE=embed` on api+worker ⇒ template files are the runtime path.
- DEV `.env` overrides SMTP to **real Gmail** (`smtp.gmail.com:587`, user `tuan.tv100698@gmail.com`) ⇒ outbound mail is delivered to real inboxes, not Mailpit.
- Deploy: cross-compiled linux api+worker (`CGO_ENABLED=0 GOOS=linux`), backed up current `bin/{api,worker}` → `*.bak.deploy_<ts>`, uploaded via scp (`.tmp` → atomic mv), recreated api+worker (`docker compose ... up -d --force-recreate --no-deps api worker`). SSH key: `~/.ssh/cobo_dev_workflow_deadline`.
- Real-send path for QA: insert `email_notifications` (status=pending) + `outbox_events` (event_type=`email.dispatch`, payload `{notification_id, variables}`) → worker `EmailDispatchHandler` resolves embed template, renders, sends via SMTP, marks `sent`. SQL: `deploy-artifacts/qa_emails.sql`.

## Runtime evidence
- `GET /healthz` = 200, `GET /readyz` = `{"status":"ready"}`, api "api listening :8080", "email delivery metrics collector registered".
- `/metrics`: `cobo_email_*` and `cobo_reminder_*` present. After QA: `cobo_email_failed_permanent_total` unchanged at 1, `cobo_email_backlog` 0.
- FE: `:3000/` = 200, deep-link `/app/ad-hoc-proposals/test-id` = 200 (SPA fallback). Routes `ad-hoc-proposals/:proposalId`, `disclosures/:id` exist in `App.tsx`.
- 5 P0 templates enqueued+sent to `tvttthptlvh@gmail.com`: all `email_notifications.status=sent`, `email_delivery_attempts` provider=smtp status=sent attempt_no=1, outbox `processed`.
- Rendered content (reproduced via embed registry, identical to deployed binary): clean Vietnamese, no `<no value>`/UUID/enum/technical code; CTAs absolute:
  - controller_review_requested → `http://88.216.208.0:3000/app/ad-hoc-proposals/prop-qa-controller-001`
  - proposal_approved → `.../app/disclosures/rec-qa-approved-001`
  - proposal_rejected → `.../app/ad-hoc-proposals/prop-qa-rejected-001` (Phase A2)
  - existing_user invitation + reminder.disclosure_deadline → text-only, portal CTA `http://88.216.208.0:3000`

## Build / verification
- `go build ./...` OK; `npm run build` OK; `go test ./internal/notification/... ./internal/reminder/... ./internal/iam/app/` all PASS.

## Remaining gaps / risks
- FE not re-pushed: zero FE source diff in this email work; existing dev `web/dist` already current; routes verified live (200). Re-deploy would be a no-op restart risk.
- Inbox-side confirmation (actually opening the 5 emails in `tvttthptlvh@gmail.com` and clicking CTA through login→company-select→target) requires the inbox owner; server-side delivery is proven (status=sent, attempt provider=smtp sent).
- Pre-existing reliability artifact: `qa-step-name-test-*` reminder occurrence stuck (backlog=1, REMINDER_SLA_BREACH) — reliability stack, out of scope, not introduced by this change.
- QA rows (`email_notifications`/`outbox_events`/`email_delivery_attempts` for the 5 sends) remain in DEV DB as normal operational data.
