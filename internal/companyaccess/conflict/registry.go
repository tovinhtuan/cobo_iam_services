package conflict

// DefaultRegistry returns MVP detectors in deterministic compile-time order.
func DefaultRegistry() []Detector {
	return []Detector{
		&OverrideStaleDetector{},
		&NotificationPrefsInvalidDetector{},
		&CriticalRoleEmptyDetector{},
		&GrantableViolationDetector{},
		&InactiveDepartmentReferencedDetector{},
		&AssigneeRoleMissingDetector{},
		&RoleUnassignedInWorkflowDetector{},
		&TierPrefsMismatchDetector{},
	}
}

// RegistryCodes returns detector codes in registry order (for tests).
func RegistryCodes() []string {
	regs := DefaultRegistry()
	out := make([]string, len(regs))
	for i, d := range regs {
		out[i] = d.Code()
	}
	return out
}
