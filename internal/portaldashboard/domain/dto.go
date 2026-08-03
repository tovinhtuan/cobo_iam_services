package domain

// OverviewResponse is the wire contract for GET /api/v1/company/dashboard/overview.
type OverviewResponse struct {
	Company            CompanyBrief           `json:"company"`
	Range              RangeInfo              `json:"range"`
	LastUpdatedAt      string                 `json:"last_updated_at"`
	Kpis               map[string]KpiMetric   `json:"kpis"`
	DeadlineHealth     DeadlineHealthBlock    `json:"deadline_health"`
	ImmediateActions   []ImmediateActionItem  `json:"immediate_actions"`
	FrequentLateFlows  []WorkflowRiskRow      `json:"frequent_late_workflows"`
	DepartmentRisks    []DepartmentRiskRow    `json:"department_risks"`
	RecentActivities   []RecentActivityItem   `json:"recent_activities"`
	Exceptions         []ExceptionItem        `json:"exceptions"`
	Meta               MetaBlock              `json:"meta"`
}

type CompanyBrief struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

type RangeInfo struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Preset   string `json:"preset"`
	Timezone string `json:"timezone"`
}

type KpiMetric struct {
	Value    *float64 `json:"value"`
	Unit     string   `json:"unit,omitempty"`
	Severity string   `json:"severity"`
	Source   *string  `json:"source"`
	Accuracy string   `json:"accuracy"`
	Reason   *string  `json:"reason"`
	// Additive on-time sample fields (dashboard KPI tooltip). Omitempty for old clients.
	CompletedOnTime *int `json:"completed_on_time,omitempty"`
	CompletedTotal  *int `json:"completed_total,omitempty"`
}

type DeadlineHealthBlock struct {
	OnTimeRate        KpiMetric       `json:"on_time_rate"`
	OnTimeCount       int             `json:"on_time_count"`
	OverdueAgeBuckets []OverdueBucket `json:"overdue_age_buckets"`
	TotalOverdue      int             `json:"total_overdue"`
	Source            string          `json:"source"`
	Accuracy          string          `json:"accuracy"`
}

type OverdueBucket struct {
	Key     string  `json:"key"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

type ImmediateActionItem struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Status          string `json:"status"`
	Severity        string `json:"severity"`
	DueDate         string `json:"due_date"`
	Reason          string `json:"reason"`
	CurrentStepName string `json:"current_step_name,omitempty"`
	Department      string `json:"department,omitempty"`
	ActionLabelKey  string `json:"action_label_key"`
	TargetURL       string `json:"target_url"`
	Source          string `json:"source"`
	Accuracy        string `json:"accuracy"`
}

type WorkflowRiskRow struct {
	Key              string   `json:"key"`
	WorkflowName     string   `json:"workflow_name"`
	OpenCount        int      `json:"open_count"`
	OverdueCount     int      `json:"overdue_count"`
	OverdueRate      *float64 `json:"overdue_rate"`
	AvgDelayDays     *float64 `json:"avg_delay_days"`
	BottleneckStep   *string  `json:"bottleneck_step"`
	OwnerDepartment  *string  `json:"owner_department"`
	Severity         string   `json:"severity"`
	Source           string   `json:"source"`
	Accuracy         string   `json:"accuracy"`
}

type DepartmentRiskRow struct {
	Key            string   `json:"key"`
	DepartmentName string   `json:"department_name"`
	TotalDue       int      `json:"total_due"`
	OverdueCount   int      `json:"overdue_count"`
	OverdueRate    *float64 `json:"overdue_rate"`
	Upcoming3Days  int      `json:"upcoming_3_days"`
	OwnerName      *string  `json:"owner_name"`
	Severity       string   `json:"severity"`
	Source         string   `json:"source"`
	Accuracy       string   `json:"accuracy"`
}

type RecentActivityItem struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Summary     string `json:"summary,omitempty"`
	OccurredAt  string `json:"occurred_at"`
	TargetURL   string `json:"target_url,omitempty"`
	Source      string `json:"source"`
	Accuracy    string `json:"accuracy"`
}

type ExceptionItem struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Summary    string `json:"summary,omitempty"`
	Severity   string `json:"severity"`
	OccurredAt string `json:"occurred_at"`
	TargetURL  string `json:"target_url,omitempty"`
	Source     string `json:"source"`
	Accuracy   string `json:"accuracy"`
}

type MetaBlock struct {
	Sources  []string `json:"sources"`
	Partial  bool     `json:"partial"`
	Warnings []string `json:"warnings"`
}
