package app

import "strings"

func firstLineOfText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return strings.TrimSpace(strings.Split(text, "\n")[0])
}

// ResolveAdHocRecordTitle is the disclosure record title after admin approval:
// proposal title (first line of change_note), else template display name, else type_id.
func ResolveAdHocRecordTitle(changeNote, typeDisplayName, typeID string) string {
	if title := firstLineOfText(changeNote); title != "" {
		return title
	}
	if name := strings.TrimSpace(typeDisplayName); name != "" {
		return name
	}
	return strings.TrimSpace(typeID)
}
