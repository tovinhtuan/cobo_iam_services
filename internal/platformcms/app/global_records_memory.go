package app

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemoryRepository is an in-memory GlobalRecords Repository for unit tests.
type MemoryRepository struct {
	mu        sync.Mutex
	records   map[string]GlobalRecord
	byCycle   map[string]string // templateID+"|"+cycleKey -> id
	templates map[string]TemplateInfo
	links     map[string]string // cmsRecordID+"|"+companyID -> recordID
	companies []string
	children  map[string][]CompanyChild // cmsRecordID -> children
	runs      []MaterializeResult
	idSeq     int
	eligibilitySkip map[string]string // companyID -> reason code
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		records:   map[string]GlobalRecord{},
		byCycle:   map[string]string{},
		templates: map[string]TemplateInfo{},
		links:     map[string]string{},
		children:  map[string][]CompanyChild{},
		companies: []string{"c_001", "c_002"},
	}
}

func (m *MemoryRepository) SeedTemplate(t TemplateInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.templates[t.TypeID] = t
}

func (m *MemoryRepository) SetCompanies(ids []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.companies = append([]string{}, ids...)
}

func cycleMapKey(templateID, cycleKey string) string {
	return templateID + "|" + cycleKey
}

func (m *MemoryRepository) Create(ctx context.Context, rec GlobalRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := cycleMapKey(rec.TemplateID, rec.CycleKey)
	if _, ok := m.byCycle[key]; ok {
		return errConflict("duplicate")
	}
	m.records[rec.ID] = rec
	m.byCycle[key] = rec.ID
	return nil
}

type memHTTPError struct {
	status  int
	message string
}

func (e memHTTPError) Error() string { return e.message }

func errConflict(msg string) error { return memHTTPError{status: 409, message: msg} }

func (m *MemoryRepository) Update(ctx context.Context, rec GlobalRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.records[rec.ID]
	if !ok {
		return memHTTPError{status: 404, message: "not found"}
	}
	// Rebuild cycle index if cycle_key changed (archive).
	oldKey := cycleMapKey(old.TemplateID, old.CycleKey)
	delete(m.byCycle, oldKey)
	m.records[rec.ID] = rec
	m.byCycle[cycleMapKey(rec.TemplateID, rec.CycleKey)] = rec.ID
	return nil
}

func (m *MemoryRepository) GetByID(ctx context.Context, id string) (*GlobalRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[id]
	if !ok {
		return nil, nil
	}
	cp := rec
	return &cp, nil
}

func (m *MemoryRepository) GetByTemplateCycle(ctx context.Context, templateID, cycleKey string) (*GlobalRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byCycle[cycleMapKey(templateID, cycleKey)]
	if !ok {
		return nil, nil
	}
	rec := m.records[id]
	cp := rec
	return &cp, nil
}

func (m *MemoryRepository) ListByTemplate(ctx context.Context, f ListGlobalRecordsFilter) (ListGlobalRecordsResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := make([]GlobalRecord, 0)
	for _, rec := range m.records {
		if rec.TemplateID != f.TemplateID {
			continue
		}
		if f.Status != "" && !strings.EqualFold(rec.Status, f.Status) {
			continue
		}
		if f.CycleKey != "" && rec.CycleKey != f.CycleKey {
			continue
		}
		if f.Query != "" && !strings.Contains(strings.ToLower(rec.Title), strings.ToLower(f.Query)) {
			continue
		}
		items = append(items, rec)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	total := len(items)
	start := (f.Page - 1) * f.PageSize
	if start > total {
		start = total
	}
	end := start + f.PageSize
	if end > total {
		end = total
	}
	return ListGlobalRecordsResult{Items: items[start:end], Page: f.Page, PageSize: f.PageSize, Total: total}, nil
}

func (m *MemoryRepository) CountCompaniesByCMSRecord(ctx context.Context, cmsRecordID string) (*CompanyCounts, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	children := m.children[cmsRecordID]
	counts := &CompanyCounts{Total: len(children)}
	for _, c := range children {
		switch MapCompanyStatusForCounts(c.Status) {
		case "not_started":
			counts.NotStarted++
		case "pending_review":
			counts.PendingReview++
		case "approved":
			counts.Approved++
		case "published":
			counts.Published++
		case "completed":
			counts.Completed++
		case "failed":
			counts.Failed++
		default:
			counts.InProgress++
		}
	}
	return counts, nil
}

func (m *MemoryRepository) ListCompanyChildren(ctx context.Context, cmsRecordID string, status string, page, pageSize int) ([]CompanyChild, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	all := m.children[cmsRecordID]
	filtered := make([]CompanyChild, 0, len(all))
	for _, c := range all {
		if status != "" && !strings.EqualFold(c.Status, status) {
			continue
		}
		filtered = append(filtered, c)
	}
	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return filtered[start:end], total, nil
}

func (m *MemoryRepository) FindCompanyRecordLink(ctx context.Context, cmsRecordID, companyID string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.links[cmsRecordID+"|"+companyID]
	return id, ok, nil
}

func (m *MemoryRepository) CreateCompanyProcessingRecord(ctx context.Context, cmsRecordID, companyID, typeID, title, summary, content, createdBy string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := cmsRecordID + "|" + companyID
	if id, ok := m.links[key]; ok {
		return id, errConflict("exists")
	}
	m.idSeq++
	recordID := "rec_mem_" + time.Now().Format("150405") + "_" + strings.TrimPrefix(companyID, "c_")
	m.links[key] = recordID
	m.children[cmsRecordID] = append(m.children[cmsRecordID], CompanyChild{
		CompanyID: companyID,
		RecordID:  recordID,
		Title:     title,
		Status:    CompanyStatusNotStarted,
		UpdatedAt: time.Now().UTC(),
	})
	return recordID, nil
}

func (m *MemoryRepository) EvaluateEligibility(ctx context.Context, templateID, cycleKey, cmsRecordID string) ([]EligibilityDecision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]EligibilityDecision, 0, len(m.companies))
	for _, id := range m.companies {
		d := EligibilityDecision{
			CompanyID:          id,
			CompanyName:        id,
			Eligible:           true,
			ReasonCode:         ReasonEligible,
			ReasonMessage:      ReasonMessageVI(ReasonEligible),
			Active:             true,
			EntitlementStatus:  "ACTIVE",
			ApplicabilityMatch: true,
			AutoCreateEnabled:  true,
			ApplicableFromOK:   true,
			ApplicableToOK:     true,
		}
		if recID, ok := m.links[cmsRecordID+"|"+id]; ok {
			d.AlreadyMaterialized = true
			d.ExistingCompanyRecID = recID
			d.Eligible = false
			d.ReasonCode = ReasonAlreadyMaterialized
			d.ReasonMessage = ReasonMessageVI(ReasonAlreadyMaterialized)
		}
		// Optional skip markers for tests
		if skip, ok := m.eligibilitySkip[id]; ok {
			d.Eligible = false
			d.ReasonCode = skip
			d.ReasonMessage = ReasonMessageVI(skip)
			d.AlreadyMaterialized = false
			switch skip {
			case ReasonCompanyInactive:
				d.Active = false
			case ReasonEntitlementMissing:
				d.EntitlementStatus = "NONE"
			case ReasonApplicabilityNotMatched:
				d.ApplicabilityMatch = false
			case ReasonAutoCreateDisabled:
				d.AutoCreateEnabled = false
			}
		}
		out = append(out, d)
	}
	return out, nil
}

func (m *MemoryRepository) SetEligibilitySkip(companyID, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.eligibilitySkip == nil {
		m.eligibilitySkip = map[string]string{}
	}
	m.eligibilitySkip[companyID] = reason
}

func (m *MemoryRepository) SaveMaterializationRun(ctx context.Context, run MaterializeResult, actor Actor, cmsRecordID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runs = append(m.runs, run)
	return nil
}

func (m *MemoryRepository) GetTemplateInfo(ctx context.Context, templateID string) (*TemplateInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.templates[templateID]
	if !ok {
		return nil, nil
	}
	cp := t
	return &cp, nil
}
