package http

const (
	cmsActionEntryCreate                   = "cms.entry.create"
	cmsActionEntryUpdate                   = "cms.entry.update"
	cmsActionReviewApprove                 = "cms.review.approve"
	cmsActionReviewReject                  = "cms.review.reject"
	cmsActionScheduleCreate                = "cms.schedule.create"
	cmsActionScheduleDelete                = "cms.schedule.delete"
	cmsActionRuleValidate                  = "cms.rule.validate"
	cmsActionSessionRevoke                 = "cms.session.revoke"
	cmsActionMediaUploadIntent             = "cms.media.upload.intent"
	cmsActionMediaUploadComplete           = "cms.media.upload.complete"
	cmsActionMediaDelete                   = "cms.media.delete"
	cmsActionDisclosureTypeVersionUpsert   = "disclosure.type.version.upsert"
	cmsActionDisclosureTypeVersionActivate = "disclosure.type.version.activate"
	cmsActionAdminUsersCreate              = "cms.admin.users.create"
	cmsActionAdminUsersInvite              = "cms.admin.users.invite"
	cmsActionAdminUsersInviteResend        = "cms.admin.users.invite.resend"
	cmsActionAdminUsersAssignCompany       = "cms.admin.users.assign_company"
	cmsActionAdminUsersPasswordReset       = "cms.admin.users.password_reset"
	cmsActionAdminMembershipCreate         = "cms.admin.membership.create"
	cmsActionSubscriptionUpgradeUpdate     = "cms.subscription_upgrade.update"
	cmsActionSubscriptionUpgradeQRUpload   = "cms.subscription_upgrade.qr.upload"
	cmsActionSubscriptionUpgradeQRDelete   = "cms.subscription_upgrade.qr.delete"
	cmsActionCompanyPlanActivate           = "cms.company_plan.activate"
	cmsActionGlobalRecordCreate            = "cms_global_record.create"
	cmsActionGlobalRecordUpdate            = "cms_global_record.update"
	cmsActionGlobalRecordPublish           = "cms_global_record.publish"
	cmsActionGlobalRecordArchive           = "cms_global_record.archive"
	cmsActionMaterializationPreview        = "cms_materialization.preview"
	cmsActionMaterializationRun            = "cms_materialization.run"
)

var cmsKnownActions = map[string]struct{}{
	cmsActionEntryCreate:                   {},
	cmsActionEntryUpdate:                   {},
	cmsActionReviewApprove:                 {},
	cmsActionReviewReject:                  {},
	cmsActionScheduleCreate:                {},
	cmsActionScheduleDelete:                {},
	cmsActionRuleValidate:                  {},
	cmsActionSessionRevoke:                 {},
	cmsActionMediaUploadIntent:             {},
	cmsActionMediaUploadComplete:           {},
	cmsActionMediaDelete:                   {},
	cmsActionDisclosureTypeVersionUpsert:   {},
	cmsActionDisclosureTypeVersionActivate: {},
	cmsActionAdminUsersCreate:              {},
	cmsActionAdminUsersInvite:              {},
	cmsActionAdminUsersInviteResend:        {},
	cmsActionAdminUsersAssignCompany:       {},
	cmsActionAdminUsersPasswordReset:       {},
	cmsActionAdminMembershipCreate:         {},
	cmsActionSubscriptionUpgradeUpdate:     {},
	cmsActionSubscriptionUpgradeQRUpload:   {},
	cmsActionSubscriptionUpgradeQRDelete:   {},
	cmsActionCompanyPlanActivate:           {},
	cmsActionGlobalRecordCreate:            {},
	cmsActionGlobalRecordUpdate:            {},
	cmsActionGlobalRecordPublish:           {},
	cmsActionGlobalRecordArchive:           {},
	cmsActionMaterializationPreview:        {},
	cmsActionMaterializationRun:            {},
}
