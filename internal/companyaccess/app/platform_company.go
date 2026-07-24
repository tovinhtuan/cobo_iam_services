package app

// CreateCompanyBootstrap optional legal/contact fields when inserting a new company (migration 0029).
// VerificationStatus defaults to "verified" for CMS in the service layer when omitted.
type CreateCompanyBootstrap struct {
	VerificationStatus string
	TaxCode              string
	RegistrationNumber string
	Address              string
	Phone                string
	ContactEmail         string
	RepresentativeName   string
}

// PlatformCompanySummary is one row for CMS company list.
type PlatformCompanySummary struct {
	CompanyID          string `json:"company_id"`
	CompanyCode        string `json:"company_code"`
	CompanyName        string `json:"company_name"`
	Status             string `json:"status"`
	VerificationStatus string `json:"verification_status"`
	TaxCode            string `json:"tax_code,omitempty"`
	RegistrationNumber string `json:"registration_number,omitempty"`
	MemberCount        int    `json:"member_count"`
	CreatedAtRFC3339   string `json:"created_at"`
	UpdatedAtRFC3339   string `json:"updated_at"`
}

// PlatformCompanyDetail is used on CMS company detail (includes aggregate stats).
type PlatformCompanyDetail struct {
	CompanyID          string `json:"company_id"`
	CompanyCode        string `json:"company_code"`
	CompanyName        string `json:"company_name"`
	Status             string `json:"status"`
	VerificationStatus string `json:"verification_status"`
	TaxCode            string `json:"tax_code,omitempty"`
	RegistrationNumber string `json:"registration_number,omitempty"`
	Address            string `json:"address,omitempty"`
	Phone              string `json:"phone,omitempty"`
	ContactEmail       string `json:"contact_email,omitempty"`
	RepresentativeName string `json:"representative_name,omitempty"`
	IsListed                      bool   `json:"is_listed"`
	IsLargePublic                 bool   `json:"is_large_public"`
	IsNonLargePublic              bool   `json:"is_non_large_public"`
	HasSubsidiaries               bool     `json:"has_subsidiaries"`
	HasSubordinateAccountingUnits bool     `json:"has_subordinate_accounting_units"`
	BusinessSectors               []string `json:"business_sectors"`
	BusinessSector                string   `json:"business_sector,omitempty"` // deprecated: first of business_sectors
	MemberCount                   int      `json:"member_count"`
	DisclosureCount    int    `json:"disclosure_count"`
	TemplateCount      int    `json:"template_count"`
	CreatedAtRFC3339   string `json:"created_at"`
	UpdatedAtRFC3339   string `json:"updated_at"`
}

type ListPlatformCompaniesRequest struct {
	Subject               AdminSubject
	Q                     string
	Status                string
	VerificationStatus    string
	Page                  int
	Limit                 int
	SortBy                string
	SortOrder             string
}

type ListPlatformCompaniesResult struct {
	Items []PlatformCompanySummary `json:"items"`
	Total int                       `json:"total"`
}

type GetPlatformCompanyRequest struct {
	Subject   AdminSubject
	CompanyID string
}

type UpdatePlatformCompanyRequest struct {
	Subject            AdminSubject
	CompanyID          string
	CompanyName        *string
	TaxCode            *string
	RegistrationNumber *string
	Address            *string
	Phone              *string
	ContactEmail       *string
	RepresentativeName *string
	VerificationStatus *string
	IsListed                      *bool
	IsLargePublic                 *bool
	IsNonLargePublic              *bool
	HasSubsidiaries               *bool
	HasSubordinateAccountingUnits *bool
	BusinessSectors               *[]string
	BusinessSector                *string // deprecated single-value write path
}

type SetPlatformCompanyStatusRequest struct {
	Subject   AdminSubject
	CompanyID string
	Status    string // e.g. active, inactive
}
