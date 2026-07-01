package app

import "time"

type CreateConfigExportRequest struct {
	Subject AdminSubject
	Modules []string
}

type ConfigExportJobView struct {
	ExportID               string    `json:"export_id"`
	SchemaVersion          string    `json:"schema_version"`
	PackageType            string    `json:"package_type"`
	Modules                []string  `json:"modules"`
	ExportedAt             time.Time `json:"exported_at"`
	ExportedByMembershipID string    `json:"exported_by_membership_id"`
	Checksum               string    `json:"checksum"`
	Warnings               []string  `json:"warnings"`
	Status                 string    `json:"status"`
}

type GetConfigExportRequest struct {
	Subject  AdminSubject
	ExportID string
}

type DownloadConfigExportRequest struct {
	Subject  AdminSubject
	ExportID string
}
