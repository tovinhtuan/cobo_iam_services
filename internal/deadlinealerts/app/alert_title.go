package app

import "strings"

// DisplayAlertTitle resolves the card title for deadline alerts (list + detail consumers).
func DisplayAlertTitle(row AlertRow) string {
	if t := strings.TrimSpace(row.AdHocTitleLine); t != "" {
		return t
	}
	typeName := strings.TrimSpace(row.TypeName)
	recordTitle := strings.TrimSpace(row.Title)
	category := strings.TrimSpace(row.TemplateCategory)

	if typeName != "" && (category == "periodic" || category == "custom") {
		if cycle := periodicCycleFromLegacyTitle(recordTitle); cycle != "" {
			return typeName + " — " + cycle
		}
		if recordTitle != "" && !strings.HasPrefix(recordTitle, "[Tự động]") {
			return recordTitle
		}
		return typeName
	}

	if cleaned := normalizeLegacyAdHocTitle(recordTitle); cleaned != "" {
		return cleaned
	}
	if typeName != "" {
		return typeName
	}
	return recordTitle
}

func normalizeLegacyAdHocTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" || strings.HasPrefix(title, "[Tự động]") {
		return ""
	}
	if strings.HasPrefix(title, "Ad-hoc: ") {
		return firstLineOfText(strings.TrimPrefix(title, "Ad-hoc: "))
	}
	return title
}

func firstLineOfText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return strings.TrimSpace(strings.Split(text, "\n")[0])
}

func periodicCycleFromLegacyTitle(title string) string {
	const prefix = "[Tự động] "
	if !strings.HasPrefix(title, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(title, prefix)
	const sep = " — "
	idx := strings.LastIndex(rest, sep)
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(rest[idx+len(sep):])
}
