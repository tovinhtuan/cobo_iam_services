package dashboard

// HealthCheck is one configuration-health check for aggregation (read-only input).
type HealthCheck struct {
	Severity string
}

// HealthView is configuration-health output for aggregation.
type HealthView struct {
	OverallStatus string
	Checks        []HealthCheck
	ScoreValue    *int
	ScoreStatus   string
}

// NotificationView is notification status output for aggregation.
type NotificationView struct {
	StorageConfigured          bool
	PayloadValid               bool
	RuntimeConsumerEnabled     bool
	SimulationAvailable        bool
	DispatchEnforcementEnabled bool
	SubscriptionTierEnforced   bool
	UIState                    string
	Warnings                   []string
}
