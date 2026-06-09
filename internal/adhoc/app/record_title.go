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

const proposalContentMaxLen = 300

// splitChangeNote extracts proposal title (first line) and content (remaining
// lines) from the change_note field. Convention: line 1 = human-readable
// title; lines 2+ = description/content, truncated to 300 chars.
func splitChangeNote(changeNote string) (title, content string) {
	changeNote = strings.TrimSpace(changeNote)
	idx := strings.IndexByte(changeNote, '\n')
	if idx < 0 {
		return changeNote, ""
	}
	title = strings.TrimSpace(changeNote[:idx])
	content = strings.TrimSpace(changeNote[idx+1:])
	if len(content) > proposalContentMaxLen {
		content = content[:proposalContentMaxLen-3] + "..."
	}
	return title, content
}
