package app

import (
	"context"
	"regexp"
	"strings"
	"time"
)

const (
	StatusDraft     = "Draft"
	StatusPublished = "Published"
	StatusArchived  = "Archived"

	CompanyStatusNotStarted = "NotStarted"

	PermRecordRead        = "cms.record.read"
	PermRecordWrite       = "cms.record.write"
	PermRecordPublish     = "cms.record.publish"
	PermRecordMaterialize = "cms.record.materialize"
	PermPlatformCMSView   = "platform.cms.view"

	AuditCreate      = "cms_global_record.create"
	AuditUpdate      = "cms_global_record.update"
	AuditPublish     = "cms_global_record.publish"
	AuditArchive     = "cms_global_record.archive"
	AuditMatPreview  = "cms_materialization.preview"
	AuditMatRun      = "cms_materialization.run"
	AuditMatItem     = "cms_materialization.item"
)

var cycleKeyPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^daily:\d{4}-\d{2}-\d{2}$`),
	regexp.MustCompile(`^weekly:\d{4}-W\d{2}$`),
	regexp.MustCompile(`^monthly:\d{4}-\d{2}$`),
	regexp.MustCompile(`^quarterly:\d{4}-Q[1-4]$`),
	regexp.MustCompile(`^yearly:\d{4}$`),
}

// ValidateCycleKey returns true for phase-1 frequency-native cycle keys.
func ValidateCycleKey(cycleKey string) bool {
	key := strings.TrimSpace(cycleKey)
	if key == "" || strings.Contains(key, "#archived:") {
		return false
	}
	for _, re := range cycleKeyPatterns {
		if re.MatchString(key) {
			return true
		}
	}
	return false
}

type GlobalRecord struct {
	ID                 string     `json:"id"`
	TemplateID         string     `json:"template_id"`
	CycleKey           string     `json:"cycle_key"`
	Title              string     `json:"title"`
	Summary            string     `json:"summary,omitempty"`
	Content            string     `json:"content"`
	Status             string     `json:"status"`
	TemplateVersionNo  *int       `json:"template_version_no,omitempty"`
	CreatedBy          string     `json:"created_by,omitempty"`
	UpdatedBy          string     `json:"updated_by,omitempty"`
	PublishedBy        string     `json:"published_by,omitempty"`
	ArchivedBy         string     `json:"archived_by,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	PublishedAt        *time.Time `json:"published_at,omitempty"`
	ArchivedAt         *time.Time `json:"archived_at,omitempty"`
	CycleStart         *time.Time `json:"cycle_start,omitempty"`
	DueDate            *time.Time `json:"due_date,omitempty"`
	CompanyCounts      *CompanyCounts `json:"company_counts,omitempty"`
}

type CompanyCounts struct {
	Total         int `json:"total"`
	NotStarted    int `json:"not_started"`
	InProgress    int `json:"in_progress"`
	PendingReview int `json:"pending_review"`
	Approved      int `json:"approved"`
	Published     int `json:"published"`
	Completed     int `json:"completed"`
	Failed        int `json:"failed"`
}

type CompanyChild struct {
	CompanyID       string     `json:"company_id"`
	RecordID        string     `json:"record_id"`
	Title           string     `json:"title"`
	Status          string     `json:"status"`
	PlannedDate     *time.Time `json:"planned_date,omitempty"`
	SubmittedAt     *time.Time `json:"submitted_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type MaterializeRequest struct {
	Mode       string   `json:"mode"` // full | incremental
	CompanyIDs []string `json:"company_ids"`
	DryRun     bool     `json:"dry_run"`
}

type MaterializeItemResult struct {
	CompanyID       string `json:"company_id"`
	CompanyName     string `json:"company_name,omitempty"`
	Outcome         string `json:"outcome"` // created|exists|skipped|failed
	CompanyRecordID string `json:"company_record_id,omitempty"`
	ErrorCode       string `json:"error_code,omitempty"`
	ErrorMessage    string `json:"error_message,omitempty"`
	ReasonCode      string `json:"reason_code,omitempty"`
}

type MaterializeResult struct {
	RunID         string                  `json:"run_id"`
	DryRun        bool                    `json:"dry_run"`
	Mode          string                  `json:"mode"`
	Status        string                  `json:"status"`
	Requested     int                     `json:"requested_count"`
	Created       int                     `json:"created_count"`
	Exists        int                     `json:"exists_count"`
	Skipped       int                     `json:"skipped_count"`
	Failed        int                     `json:"failed_count"`
	EligibleCount int                     `json:"eligible_count"`
	Results       []MaterializeItemResult `json:"results"`
	Eligibility   []EligibilityDecision   `json:"eligibility,omitempty"`
}

type ListGlobalRecordsFilter struct {
	TemplateID string
	Status     string
	CycleKey   string
	Query      string
	Page       int
	PageSize   int
}

type ListGlobalRecordsResult struct {
	Items    []GlobalRecord `json:"items"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Total    int            `json:"total"`
}

type Actor struct {
	UserID       string
	MembershipID string
	CompanyID    string // CMS subject company context (audit only; not tenant scope for global)
}

type TemplateInfo struct {
	TypeID            string
	Scope             string // global / company
	ActiveVersionNo   int
	PortalState       string // active / archived / ...
	ReviewStatus      string
	TemplateCategory  string
	Periodicity       string
}

type Repository interface {
	Create(ctx context.Context, rec GlobalRecord) error
	Update(ctx context.Context, rec GlobalRecord) error
	GetByID(ctx context.Context, id string) (*GlobalRecord, error)
	GetByTemplateCycle(ctx context.Context, templateID, cycleKey string) (*GlobalRecord, error)
	ListByTemplate(ctx context.Context, f ListGlobalRecordsFilter) (ListGlobalRecordsResult, error)
	CountCompaniesByCMSRecord(ctx context.Context, cmsRecordID string) (*CompanyCounts, error)
	ListCompanyChildren(ctx context.Context, cmsRecordID string, status string, page, pageSize int) ([]CompanyChild, int, error)
	FindCompanyRecordLink(ctx context.Context, cmsRecordID, companyID string) (recordID string, found bool, err error)
	CreateCompanyProcessingRecord(ctx context.Context, cmsRecordID, companyID, typeID, title, summary, content, createdBy string) (recordID string, err error)
	// EvaluateEligibility resolves full eligibility for template+cycle (active, entitlement, applicability, auto_create, applicable_from/to).
	EvaluateEligibility(ctx context.Context, templateID, cycleKey, cmsRecordID string) ([]EligibilityDecision, error)
	SaveMaterializationRun(ctx context.Context, run MaterializeResult, actor Actor, cmsRecordID string) error
	GetTemplateInfo(ctx context.Context, templateID string) (*TemplateInfo, error)
}

type IDGenerator interface {
	NewID(prefix string) string
}

type Clock interface {
	Now() time.Time
}
