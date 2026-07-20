package domain

// OverviewResponse is the wire contract for GET /api/v1/me/operational-overview.
type OverviewResponse struct {
	Profile          ProfileBrief         `json:"profile"`
	Kpis             KpiBlock             `json:"kpis"`
	CompanyOverviews []CompanyOverview    `json:"company_overviews"`
	MyTasks          []MyTaskItem         `json:"my_tasks"`
	RoleAssignments  []RoleAssignment     `json:"role_assignments"`
	AdminScopes      []AdminScope         `json:"admin_scopes"`
	Activities       []ActivityItem       `json:"activities"`
	ActivityLog      []ActivityItem       `json:"activity_log"`
	Meta             MetaBlock            `json:"meta"`
}

type ProfileBrief struct {
	UserID           string  `json:"user_id"`
	DisplayName      string  `json:"display_name"`
	Email            string  `json:"email"`
	Phone            string  `json:"phone"`
	AvatarURL        *string `json:"avatar_url"`
	SubscriptionTier string  `json:"subscription_tier"`
	EmailVerified    bool    `json:"email_verified"`
}

type Metric struct {
	Value           *int     `json:"value"`
	Accuracy        string   `json:"accuracy"`
	Reason          *string  `json:"reason,omitempty"`
	DeadlineSources []string `json:"deadline_sources,omitempty"`
}

type RateMetric struct {
	Value           *float64 `json:"value"`
	Accuracy        string   `json:"accuracy"`
	Reason          *string  `json:"reason,omitempty"`
	SampleSize      int      `json:"sample_size"`
	CompletedOnTime int      `json:"completed_on_time"`
	CompletedTotal  int      `json:"completed_total"`
	Source          *string  `json:"source"`
}

type KpiBlock struct {
	LinkedCompanies Metric `json:"linked_companies"`
	ActiveRoles     Metric `json:"active_roles"`
	AssignedAlerts  Metric `json:"assigned_alerts"`
	OverdueAlerts   Metric `json:"overdue_alerts"`
}

type CompanyOverview struct {
	CompanyID       string     `json:"company_id"`
	CompanyName     string     `json:"company_name"`
	AssignedAlerts  Metric     `json:"assigned_alerts"`
	OverdueAlerts   Metric     `json:"overdue_alerts"`
	DueSoonAlerts   Metric     `json:"due_soon_alerts"`
	CompletedAlerts Metric     `json:"completed_alerts"`
	OnTimeRate      RateMetric `json:"on_time_rate"`
}

type MyTaskAction struct {
	Label string `json:"label"`
	Href  string `json:"href"`
}

type MyTaskItem struct {
	ID                 string       `json:"id"`
	CompanyID          string       `json:"company_id"`
	CompanyName        string       `json:"company_name"`
	AlertID            string       `json:"alert_id"`
	AlertTitle         string       `json:"alert_title"`
	MyRoleLabel        string       `json:"my_role_label"`
	CurrentStepLabel   string       `json:"current_step_label"`
	Deadline           string       `json:"deadline"`
	DeadlineSource     string       `json:"deadline_source"`
	DeadlineAccuracy   string       `json:"deadline_accuracy"`
	Status             string       `json:"status"`
	StatusLabel        string       `json:"status_label"`
	Action             MyTaskAction `json:"action"`
}

type RoleAssignment struct {
	CompanyID      string   `json:"company_id"`
	CompanyName    string   `json:"company_name"`
	MembershipID   string   `json:"membership_id"`
	DepartmentName *string  `json:"department_name"`
	TitleNames     []string `json:"title_names"`
	RoleNames      []string `json:"role_names"`
	Status         string   `json:"status"`
}

type AdminScope struct {
	CompanyID   string  `json:"company_id"`
	CompanyName string  `json:"company_name"`
	CanView     *bool   `json:"can_view"`
	CanEdit     *bool   `json:"can_edit"`
	CanManage   *bool   `json:"can_manage"`
	Href        *string `json:"href"`
	Note        *string `json:"note,omitempty"`
}

type ActivityItem struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Description  string `json:"description,omitempty"`
	CreatedAt    string `json:"created_at"`
	Href         string `json:"href"`
	Source       string `json:"source"`
	Action       string `json:"action,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
	ResourceID   string `json:"resource_id,omitempty"`
	Domain       string `json:"domain,omitempty"`
}

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type MetaBlock struct {
	GeneratedAt string    `json:"generated_at"`
	Partial     bool      `json:"partial"`
	Warnings    []Warning `json:"warnings"`
	Sources     []string  `json:"sources"`
}
