package app

import disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"

// ObligationOpenSQL is the shared ObligationOpen predicate (disclosure/app). The template's state
// is not part of it (PO decision Q1-B). One bind argument: todayHCM.
const ObligationOpenSQL = disclosureapp.ObligationOpenSQL

// TemplateArchivedSQL is true when the template joined as dt is archived (disclosure/app).
const TemplateArchivedSQL = disclosureapp.TemplateArchivedSQL

// TemplateArchived is the in-process equivalent of TemplateArchivedSQL.
func TemplateArchived(status, companyID, reviewStatus string) bool {
	return disclosureapp.TemplateArchived(status, companyID, reviewStatus)
}

// ObligationTemplateSQLJoin joins a record's template for display only (name, category, archived
// flag); it never filters rows. The version is the active one, else the one the template was
// archived from, so an archived template keeps its name and deadline config.
const ObligationTemplateSQLJoin = `
		LEFT JOIN disclosure_types dt ON dt.type_id = dr.type_id
		LEFT JOIN disclosure_type_versions dtv ON dtv.type_id = dt.type_id
			AND dtv.version_no = COALESCE(NULLIF(dt.active_version_no, 0), dt.archived_from_version_no)
`
