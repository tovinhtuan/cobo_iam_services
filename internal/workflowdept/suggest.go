package workflowdept

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

type SuggestionClass string

const (
	SuggestExact     SuggestionClass = "EXACT_EQUIVALENT"
	SuggestPrefix    SuggestionClass = "PREFIX_VARIANT"
	SuggestUnicode   SuggestionClass = "UNICODE_VARIANT"
	SuggestDiacritic SuggestionClass = "DIACRITIC_VARIANT"
	SuggestSemantic  SuggestionClass = "SEMANTIC_SIMILAR"
	SuggestNone      SuggestionClass = "NONE"
)

// ClassifySuggestion is for backfill reports only. Runtime resolution must not call it.
func ClassifySuggestion(catalogName, companyName string) SuggestionClass {
	a := fold(catalogName)
	b := fold(companyName)
	if a == "" || b == "" {
		return SuggestNone
	}
	if a == b {
		if norm.NFC.String(catalogName) != catalogName || norm.NFC.String(companyName) != companyName {
			return SuggestUnicode
		}
		return SuggestExact
	}
	if stripPrefix(a) == stripPrefix(b) && stripPrefix(a) != "" {
		return SuggestPrefix
	}
	if stripMarks(a) == stripMarks(b) {
		return SuggestDiacritic
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return SuggestSemantic
	}
	return SuggestNone
}

// AllowAutoWrite is the only gate that may persist a backfill row.
func AllowAutoWrite(flag bool, class SuggestionClass, candidateCount int) bool {
	return flag && BackfillWriteEnabled() && class == SuggestExact && candidateCount == 1
}

func fold(s string) string {
	s = norm.NFKC.String(strings.TrimSpace(s))
	s = strings.Join(strings.Fields(s), " ")
	return strings.ToLower(s)
}

func stripPrefix(s string) string {
	for _, p := range []string{"phòng ", "phong ", "ban "} {
		if strings.HasPrefix(s, p) {
			return strings.TrimSpace(strings.TrimPrefix(s, p))
		}
	}
	return s
}

func stripMarks(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
