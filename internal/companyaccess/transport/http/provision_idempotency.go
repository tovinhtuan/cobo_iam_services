package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

type companyProvisionBody struct {
	CompanyName        string `json:"company_name"`
	TaxCode            string `json:"tax_code"`
	RegistrationNumber string `json:"registration_number"`
	Address            string `json:"address"`
	Phone              string `json:"phone"`
	ContactEmail       string `json:"contact_email"`
}

func companyProvisionRequestHash(userID string, body companyProvisionBody, op string) string {
	canonical := canonicalProvisionJSON(body)
	h := sha256.Sum256([]byte(canonical + "|" + strings.TrimSpace(userID) + "|" + op))
	return hex.EncodeToString(h[:])
}

func canonicalProvisionJSON(body companyProvisionBody) string {
	m := map[string]string{}
	if v := strings.TrimSpace(body.CompanyName); v != "" {
		m["company_name"] = v
	}
	if v := strings.TrimSpace(body.TaxCode); v != "" {
		m["tax_code"] = v
	}
	if v := strings.TrimSpace(body.RegistrationNumber); v != "" {
		m["registration_number"] = v
	}
	if v := strings.TrimSpace(body.Address); v != "" {
		m["address"] = v
	}
	if v := strings.TrimSpace(body.Phone); v != "" {
		m["phone"] = v
	}
	if v := strings.TrimSpace(body.ContactEmail); v != "" {
		m["contact_email"] = v
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		out[k] = m[k]
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func idempotencyConflictBody(msg string) map[string]any {
	return map[string]any{
		"error": map[string]any{
			"code":    "IDEMPOTENCY_CONFLICT",
			"message": msg,
		},
	}
}
