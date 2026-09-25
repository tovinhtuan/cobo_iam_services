package workflowdept

import (
	"os"
	"strings"
)

// Flags default off. Disabled mode keeps legacy exact department id/code matching.
const (
	EnvBindingEnabled       = "WORKFLOW_DEPARTMENT_BINDING_ENABLED"
	EnvEmailBindingEnabled  = "WORKFLOW_DEPARTMENT_EMAIL_BINDING_ENABLED"
	EnvTaskRoutingEnabled   = "WORKFLOW_DEPARTMENT_TASK_ROUTING_ENABLED"
	EnvBackfillWriteEnabled = "WORKFLOW_DEPARTMENT_BACKFILL_WRITE_ENABLED"
)

func envOn(key string) bool {
	v := strings.TrimSpace(os.Getenv(key))
	return strings.EqualFold(v, "1") || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

func BindingEnabled() bool       { return envOn(EnvBindingEnabled) }
func EmailBindingEnabled() bool  { return BindingEnabled() && envOn(EnvEmailBindingEnabled) }
func TaskRoutingEnabled() bool   { return BindingEnabled() && envOn(EnvTaskRoutingEnabled) }
func BackfillWriteEnabled() bool { return envOn(EnvBackfillWriteEnabled) }
