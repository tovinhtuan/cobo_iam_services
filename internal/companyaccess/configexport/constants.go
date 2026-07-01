package configexport

const (
	SchemaVersionEnterpriseExport = "enterprise_config_export.v1"
	PackageTypeEnterpriseExport   = "enterprise_config_export"
	SourceActiveConfiguration     = "active_configuration"

	ModuleNotificationAlertChannelPrefs = "notification.alert_channel_prefs"
	ModuleRBACMatrix                    = "rbac.matrix"
)

// DefaultModules is the canonical MVP module order (deterministic).
var DefaultModules = []string{
	ModuleNotificationAlertChannelPrefs,
	ModuleRBACMatrix,
}
