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
}
