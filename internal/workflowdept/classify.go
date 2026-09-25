package workflowdept

import "strings"

type TokenKind string

const (
	TokenTemplateDepartmentCode TokenKind = "TEMPLATE_DEPARTMENT_CODE"
	TokenCompanyDepartmentID    TokenKind = "COMPANY_DEPARTMENT_ID"
	TokenCompanyDepartmentCode  TokenKind = "COMPANY_DEPARTMENT_CODE"
	TokenLegacyLabel            TokenKind = "LEGACY_LABEL"
	TokenUnknown                TokenKind = "UNKNOWN"
)

// DeptRef is an active company department used only for classification.
type DeptRef struct {
	ID   string
	Code string
}

// Classify picks a token kind. Catalog membership wins over a company code
// that happens to equal the same string (for example dept-001).
func Classify(token string, catalogCodes map[string]struct{}, company []DeptRef, technical func(string) bool) TokenKind {
	token = strings.TrimSpace(token)
	if token == "" {
		return TokenUnknown
	}
	if _, ok := catalogCodes[token]; ok {
		return TokenTemplateDepartmentCode
	}
	low := strings.ToLower(token)
	for code := range catalogCodes {
		if strings.EqualFold(code, token) {
			return TokenTemplateDepartmentCode
		}
	}
	for _, d := range company {
		if strings.TrimSpace(d.ID) == token || strings.EqualFold(strings.TrimSpace(d.ID), token) {
			return TokenCompanyDepartmentID
		}
	}
	for _, d := range company {
		code := strings.TrimSpace(d.Code)
		if code != "" && (code == token || strings.EqualFold(code, low) || strings.EqualFold(code, token)) {
			return TokenCompanyDepartmentCode
		}
	}
	if technical != nil && technical(token) {
		return TokenUnknown
	}
	return TokenLegacyLabel
}
