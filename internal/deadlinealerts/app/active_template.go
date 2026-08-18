package app

import "strings"

// ListRowsActiveTemplateSQLJoin is the CURRENT_TEMPLATE_ACTIVE_STATE predicate
// for GET /api/v1/company/deadline-alerts ListRows.
//
// Authority matches Portal ListTypes (disclosure ListTypes INNER JOIN versions
// on disclosure_types.active_version_no) and CMS archive
// (status=archived AND active_version_no=0).
//
// Global (company_id IS NULL) and company-scoped roots share disclosure_types.
// Workflow overrides are NOT template active state.
const ListRowsActiveTemplateSQLJoin = `
		INNER JOIN disclosure_types dt ON dt.type_id = dr.type_id
			AND dt.active_version_no > 0
		INNER JOIN disclosure_type_versions dtv ON dtv.type_id = dt.type_id AND dtv.version_no = dt.active_version_no
`

// TemplateCurrentlyActive is the in-process equivalent of ListRowsActiveTemplateSQLJoin.
// Missing type_id, archived/inactive (active_version_no=0), or missing version row → not visible.
func TemplateCurrentlyActive(typeID string, activeVersionNo int, versionRowExists bool) bool {
	if strings.TrimSpace(typeID) == "" {
		return false
	}
	if activeVersionNo <= 0 {
		return false
	}
	return versionRowExists
}
