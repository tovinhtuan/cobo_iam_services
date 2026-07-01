package app

import (
	"sync"
	"time"
)

type configExportJob struct {
	ExportID               string
	CompanyID              string
	ExportedByMembershipID string
	SchemaVersion          string
	PackageType            string
	Modules                []string
	Checksum               string
	Warnings               []string
	ArtifactJSON           []byte
	CreatedAt              time.Time
	Status                 string
}

type configExportStore struct {
	mu   sync.RWMutex
	jobs map[string]*configExportJob
}

func newConfigExportStore() *configExportStore {
	return &configExportStore{jobs: make(map[string]*configExportJob)}
}

func (s *configExportStore) put(job *configExportJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ExportID] = job
}

func (s *configExportStore) get(companyID, exportID string) (*configExportJob, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[exportID]
	if !ok || job.CompanyID != companyID {
		return nil, false
	}
	return job, true
}
