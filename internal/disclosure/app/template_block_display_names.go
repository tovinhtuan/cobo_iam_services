package app

import "strings"

// HydrateTemplateBlocksBilingualForPersistence fills name_en / name_vi before persistence when the client only sends legacy `title`.
func HydrateTemplateBlocksBilingualForPersistence(blocks []TemplateBlockDTO) {
	for i := range blocks {
		b := &blocks[i]
		title := strings.TrimSpace(b.Title)
		if strings.TrimSpace(b.NameVI) == "" && title != "" {
			b.NameVI = title
		}
		if strings.TrimSpace(b.NameEN) == "" {
			if en := MandatoryBlockTitleEnglish(strings.TrimSpace(b.BlockKey)); en != "" {
				b.NameEN = en
			}
		}
	}
}

// MandatoryBlockTitleEnglish returns default English headline for mandatory block_key values.
func MandatoryBlockTitleEnglish(blockKey string) string {
	switch strings.ToLower(strings.TrimSpace(blockKey)) {
	case "legal_basis":
		return "Legal basis"
	case "disclosure_content":
		return "Disclosure and reporting content"
	case "deadline":
		return "Disclosure/reporting deadline"
	case "channels_and_format":
		return "Channels and format"
	case "legal_risks":
		return "Legal risks if requirements are not met"
	case "enterprise_workflow":
		return "Enterprise workflow"
	default:
		return ""
	}
}

// EnrichTemplateBlockDisplayNames sets name_en / name_vi on read models when the DB only stores legacy `title`.
// Call on HTTP read path only; does not mutate persisted data or block_key.
func EnrichTemplateBlockDisplayNames(blocks []TemplateBlockDTO) {
	for i := range blocks {
		b := &blocks[i]
		k := strings.ToLower(strings.TrimSpace(b.BlockKey))
		if k == "" {
			continue
		}
		title := strings.TrimSpace(b.Title)
		if strings.TrimSpace(b.NameVI) == "" {
			if title != "" {
				b.NameVI = title
			} else {
				vi, _, _ := mandatoryBlockShape(k)
				if strings.TrimSpace(vi) != "" && vi != k {
					b.NameVI = vi
				}
			}
		}
		if strings.TrimSpace(b.NameEN) == "" {
			if en := MandatoryBlockTitleEnglish(k); en != "" {
				b.NameEN = en
			}
		}
	}
}
