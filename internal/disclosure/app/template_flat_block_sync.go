package app

import (
	"fmt"
	"strings"

	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// ApplyTemplateFlatBlockSync runs before validateTemplateMatrix:
// 1) If blocks is empty but flat narrative fields are filled, materialize six mandatory blocks.
// 2) Rebuild block list: six mandatory keys first (fixed order), then extras; merge flat columns into block.Description (flat wins when non-empty).
// 3) Copy mandatory block descriptions back onto UpsertTypeVersionRequest flat fields so DB columns stay aligned with the matrix.
//
// Empty blocks and empty flat fields remain unchanged (legacy "no matrix" upserts).
//
// DEPRECATED / NO-OP for the "enterprise_workflow" mandatory key specifically (Architecture
// Integrity Fix D, see docs/ai-cache/workflow-architecture-integrity-fix/):
// this key's Description round-trips losslessly via req.ImplementationContent on every save, but
// has zero render site in any current CMS UI (confirmed: no .tsx file in
// cobo_web_design/src/features/cms-core/ binds to form.enterpriseWorkflow). The workflow STEPS
// that actually matter to runtime live in the SAME content block's Config.steps field, which this
// sync never touches and which has its own dead editor (EnterpriseWorkflowSection.tsx, unmounted).
// Accepted as P3 tech debt rather than removed: the round-trip is harmless (no admin can observe
// or be misled by it today, since no UI surfaces it) and removing it would require coordinated
// changes across both repos (templateMappers.ts, cmsApi.ts, types.ts, templateValidation.ts,
// templateDefaults.ts, useTemplateEditor.ts) for a field that causes no runtime-correctness bug —
// disproportionate to the issue for this fix's scope. Revisit if/when the CMS template editor's
// mandatory-blocks UI is next touched.
func ApplyTemplateFlatBlockSync(req *UpsertTypeVersionRequest, idg idgen.Generator) {
	if req == nil || idg == nil {
		return
	}
	ensureMandatoryBlocksFromFlatWhenBlocksEmpty(req, idg)
	if len(req.Blocks) == 0 {
		return
	}
	rebuildMandatoryFirstThenExtras(req, idg)
	mirrorMandatoryBlockDescriptionsToFlatFields(req)
}

func anyMandatoryFlatFieldSet(req *UpsertTypeVersionRequest) bool {
	if req == nil {
		return false
	}
	if strings.TrimSpace(req.LegalBasis) != "" {
		return true
	}
	if strings.TrimSpace(req.ReportContent) != "" {
		return true
	}
	if strings.TrimSpace(req.DeadlineRule) != "" {
		return true
	}
	if strings.TrimSpace(req.ChannelsText) != "" {
		return true
	}
	if strings.TrimSpace(req.Format) != "" {
		return true
	}
	if strings.TrimSpace(req.LegalRisksText) != "" {
		return true
	}
	if strings.TrimSpace(req.ImplementationContent) != "" {
		return true
	}
	return false
}

func ensureMandatoryBlocksFromFlatWhenBlocksEmpty(req *UpsertTypeVersionRequest, idg idgen.Generator) {
	if len(req.Blocks) > 0 || !anyMandatoryFlatFieldSet(req) {
		return
	}
	order := 1
	for _, mk := range MandatoryTemplateBlockKeys {
		req.Blocks = append(req.Blocks, newMandatoryBlockFromFlatKey(mk, req, idg, order))
		order++
	}
}

func hasDuplicateBlockKeys(blocks []TemplateBlockDTO) bool {
	seen := map[string]struct{}{}
	for _, b := range blocks {
		k := strings.ToLower(strings.TrimSpace(b.BlockKey))
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			return true
		}
		seen[k] = struct{}{}
	}
	return false
}

func rebuildMandatoryFirstThenExtras(req *UpsertTypeVersionRequest, idg idgen.Generator) {
	if hasDuplicateBlockKeys(req.Blocks) {
		return
	}
	byMandatory := map[string]TemplateBlockDTO{}
	var extras []TemplateBlockDTO
	for _, b := range req.Blocks {
		k := strings.ToLower(strings.TrimSpace(b.BlockKey))
		if k == "" {
			continue
		}
		if isMandatoryBlockKey(k) {
			byMandatory[k] = b
			continue
		}
		extras = append(extras, b)
	}
	out := make([]TemplateBlockDTO, 0, len(MandatoryTemplateBlockKeys)+len(extras))
	order := 1
	for _, mk := range MandatoryTemplateBlockKeys {
		b, ok := byMandatory[mk]
		if !ok {
			b = newMandatoryBlockFromFlatKey(mk, req, idg, order)
		} else {
			applyFlatStringToMandatoryDescription(mk, &b, req)
			if strings.TrimSpace(b.BlockID) == "" {
				b.BlockID = idg.NewUUID()
			}
		}
		b.DisplayOrder = order
		order++
		out = append(out, b)
	}
	for _, b := range extras {
		if strings.TrimSpace(b.BlockID) == "" {
			b.BlockID = idg.NewUUID()
		}
		b.DisplayOrder = order
		order++
		out = append(out, b)
	}
	req.Blocks = out
}

func isMandatoryBlockKey(k string) bool {
	k = strings.ToLower(strings.TrimSpace(k))
	for _, mk := range MandatoryTemplateBlockKeys {
		if mk == k {
			return true
		}
	}
	return false
}

func applyFlatStringToMandatoryDescription(mk string, b *TemplateBlockDTO, req *UpsertTypeVersionRequest) {
	s := flatStringForMandatoryKey(mk, req)
	if strings.TrimSpace(s) == "" {
		return
	}
	b.Description = s
}

func flatStringForMandatoryKey(mk string, req *UpsertTypeVersionRequest) string {
	switch mk {
	case "legal_basis":
		return req.LegalBasis
	case "disclosure_content":
		return req.ReportContent
	case "deadline":
		return req.DeadlineRule
	case "channels_and_format":
		return channelsAndFormatFlat(req)
	case "legal_risks":
		return req.LegalRisksText
	case "enterprise_workflow":
		return req.ImplementationContent
	default:
		return ""
	}
}

func channelsAndFormatFlat(req *UpsertTypeVersionRequest) string {
	var parts []string
	if strings.TrimSpace(req.ChannelsText) != "" {
		parts = append(parts, strings.TrimSpace(req.ChannelsText))
	}
	if strings.TrimSpace(req.Format) != "" {
		parts = append(parts, "Format: "+strings.TrimSpace(req.Format))
	}
	return strings.Join(parts, "\n")
}

func newMandatoryBlockFromFlatKey(mk string, req *UpsertTypeVersionRequest, idg idgen.Generator, displayOrder int) TemplateBlockDTO {
	title, bt, cfg := mandatoryBlockShape(mk)
	return TemplateBlockDTO{
		BlockID:      idg.NewUUID(),
		BlockKey:     mk,
		BlockType:    bt,
		Title:        title,
		Description:  strings.TrimSpace(flatStringForMandatoryKey(mk, req)),
		Config:       cfg,
		Validation:   map[string]any{},
		DisplayOrder: displayOrder,
		Enabled:      true,
	}
}

func mandatoryBlockShape(mk string) (title string, blockType string, config map[string]any) {
	switch mk {
	case "legal_basis":
		return "Cơ sở pháp lý", "rich_text", map[string]any{"max_length": 8000, "allow_html": false}
	case "disclosure_content":
		return "Nội dung công bố/báo cáo", "rich_text", map[string]any{"max_length": 50000, "allow_html": true}
	case "deadline":
		return "Kỳ hạn công bố/báo cáo", "text", map[string]any{"max_length": 4000}
	case "channels_and_format":
		return "Kênh và hình thức công bố/báo cáo", "rich_text", map[string]any{"max_length": 12000, "allow_html": false}
	case "legal_risks":
		return "Rủi ro pháp lý nếu không thực hiện đúng", "rich_text", map[string]any{"max_length": 8000, "allow_html": false}
	case "enterprise_workflow":
		return "Workflow của doanh nghiệp", "rich_text", map[string]any{"max_length": 12000, "allow_html": true}
	default:
		return mk, "text", map[string]any{}
	}
}

func mirrorMandatoryBlockDescriptionsToFlatFields(req *UpsertTypeVersionRequest) {
	byKey := map[string]*TemplateBlockDTO{}
	for i := range req.Blocks {
		k := strings.ToLower(strings.TrimSpace(req.Blocks[i].BlockKey))
		if k != "" {
			byKey[k] = &req.Blocks[i]
		}
	}
	for _, mk := range MandatoryTemplateBlockKeys {
		b := byKey[mk]
		if b == nil {
			continue
		}
		d := strings.TrimSpace(b.Description)
		switch mk {
		case "legal_basis":
			req.LegalBasis = d
		case "disclosure_content":
			req.ReportContent = d
		case "deadline":
			req.DeadlineRule = d
		case "channels_and_format":
			channelLines, formatCSV := channelsAndFormatFromConfig(b.Config)
			if strings.TrimSpace(channelLines) != "" {
				req.ChannelsText = channelLines
			} else {
				req.ChannelsText = d
			}
			if strings.TrimSpace(formatCSV) != "" {
				req.Format = formatCSV
			}
		case "legal_risks":
			req.LegalRisksText = d
		case "enterprise_workflow":
			req.ImplementationContent = d
		}
	}
}

func channelsAndFormatFromConfig(config map[string]any) (string, string) {
	if config == nil {
		return "", ""
	}
	var channels []string
	if rawChannels, ok := config["channels"].([]any); ok {
		for _, item := range rawChannels {
			if row, ok := item.(map[string]any); ok {
				if name := strings.TrimSpace(fmt.Sprint(row["name"])); name != "" && name != "<nil>" {
					channels = append(channels, name)
				}
			}
		}
	}
	var fileTypes []string
	if rawFileTypes, ok := config["file_types"].([]any); ok {
		for _, item := range rawFileTypes {
			value := strings.ToUpper(strings.TrimSpace(fmt.Sprint(item)))
			if value == "" || value == "<nil>" {
				continue
			}
			fileTypes = append(fileTypes, value)
		}
	}
	return strings.Join(channels, "\n"), strings.Join(fileTypes, ",")
}
