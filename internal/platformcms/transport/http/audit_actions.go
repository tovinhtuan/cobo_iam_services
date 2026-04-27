package http

const (
	cmsActionEntryCreate    = "cms.entry.create"
	cmsActionEntryUpdate    = "cms.entry.update"
	cmsActionReviewApprove  = "cms.review.approve"
	cmsActionReviewReject   = "cms.review.reject"
	cmsActionScheduleCreate = "cms.schedule.create"
	cmsActionScheduleDelete = "cms.schedule.delete"
	cmsActionRuleValidate   = "cms.rule.validate"
	cmsActionSessionRevoke  = "cms.session.revoke"
)

var cmsKnownActions = map[string]struct{}{
	cmsActionEntryCreate:    {},
	cmsActionEntryUpdate:    {},
	cmsActionReviewApprove:  {},
	cmsActionReviewReject:   {},
	cmsActionScheduleCreate: {},
	cmsActionScheduleDelete: {},
	cmsActionRuleValidate:   {},
	cmsActionSessionRevoke:  {},
}
