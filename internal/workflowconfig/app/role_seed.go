package app

// CanonicalRoleSeed returns the canonical workflow assignee-role definitions.
//
// Provenance (see Batch 3 report):
//   FOUND_IN_SOURCE     — role/alias exists in current code or seed migrations.
//   PARTIAL_IN_SOURCE   — the action/concept exists at runtime but no seeded role row.
//   NOT_FOUND_IN_SOURCE — named only by the approved baseline (Phase 13A), not yet in source.
//
// No roles are invented beyond the baseline-named set. Aliases capture the FE ids
// (templateWorkflowCatalog.ts) and BE defaults so the registry resolves existing data.
func CanonicalRoleSeed() []RoleDefinition {
	return []RoleDefinition{
		// ── system_owned ───────────────────────────────────────────────────────
		{
			RoleID: "disclosure_owner", Code: "disclosure_owner", Name: "Chủ hồ sơ công bố",
			Class: RoleClassSystemOwned, Locked: true, AllowedOverride: false, IsApprovalRole: false,
			Description: "The disclosure record owner (creator). System-assigned; tenant cannot reassign.",
			AllowedAssignmentScopes: []string{ScopeSystem, ScopeCompanyDefault},
			Aliases:                 []string{"owner", "requester"}, // FOUND_IN_SOURCE: 'owner' role; record created_by
		},
		{
			RoleID: "system_reviewer", Code: "system_reviewer", Name: "System Reviewer",
			Class: RoleClassSystemOwned, Locked: true, AllowedOverride: false, IsApprovalRole: false,
			Description: "Platform/automated review actor. NOT_FOUND_IN_SOURCE — baseline-defined (Phase 13A).",
			AllowedAssignmentScopes: []string{ScopeSystem},
		},
		// ── protected ──────────────────────────────────────────────────────────
		{
			RoleID: "approver", Code: "approver", Name: "Người phê duyệt",
			Class: RoleClassProtected, Locked: false, Mandatory: true, AllowedOverride: true, IsApprovalRole: true,
			Description: "Approval role. At least one approval role must exist in a workflow (mandatory).",
			AllowedAssignmentScopes: []string{ScopeUser, ScopeDepartment, ScopeCompanyDefault},
			Aliases:                 []string{"role-approver"}, // FOUND_IN_SOURCE (FE catalog; workflow.approve)
		},
		{
			RoleID: "compliance_officer", Code: "compliance_officer", Name: "Cán bộ tuân thủ",
			Class: RoleClassProtected, Locked: true, AllowedOverride: false, IsApprovalRole: false,
			Description: "Compliance officer. NOT_FOUND_IN_SOURCE as a seeded role ('compliance' appears in display groups only).",
			AllowedAssignmentScopes: []string{ScopeUser, ScopeDepartment, ScopeCompanyDefault},
		},
		// ── standard ───────────────────────────────────────────────────────────
		{
			RoleID: "reviewer", Code: "reviewer", Name: "Người rà soát",
			Class: RoleClassStandard, Locked: false, AllowedOverride: true, IsApprovalRole: false,
			Description: "Review role.",
			AllowedAssignmentScopes: []string{ScopeUser, ScopeDepartment, ScopeGroup, ScopeCompanyDefault},
			Aliases:                 []string{"role-reviewer", "disclosure_reviewer"}, // FOUND_IN_SOURCE
		},
		{
			RoleID: "confirmer", Code: "confirmer", Name: "Người xác nhận",
			Class: RoleClassStandard, Locked: false, AllowedOverride: true, IsApprovalRole: false,
			Description: "Confirm role. PARTIAL_IN_SOURCE — workflow.confirm action exists; no seeded role.",
			AllowedAssignmentScopes: []string{ScopeUser, ScopeDepartment, ScopeGroup, ScopeCompanyDefault},
		},
		{
			RoleID: "legal_reviewer", Code: "legal_reviewer", Name: "Pháp chế rà soát",
			Class: RoleClassStandard, Locked: false, AllowedOverride: true, IsApprovalRole: false,
			Description: "Legal review role.",
			AllowedAssignmentScopes: []string{ScopeUser, ScopeDepartment, ScopeGroup, ScopeCompanyDefault},
			Aliases:                 []string{"role-legal-reviewer"}, // FOUND_IN_SOURCE (FE catalog)
		},
		{
			RoleID: "finance_reviewer", Code: "finance_reviewer", Name: "Kế toán rà soát",
			Class: RoleClassStandard, Locked: false, AllowedOverride: true, IsApprovalRole: false,
			Description: "Finance review role.",
			AllowedAssignmentScopes: []string{ScopeUser, ScopeDepartment, ScopeGroup, ScopeCompanyDefault},
			Aliases:                 []string{"role-finance-reviewer"}, // FOUND_IN_SOURCE (FE catalog)
		},
	}
}
