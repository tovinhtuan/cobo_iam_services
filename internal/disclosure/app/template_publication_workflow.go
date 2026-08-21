package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	WorkflowAuthorityLegacyCompat   = "LEGACY_COMPAT"
	WorkflowAuthorityTemplatePinned = "TEMPLATE_PINNED"
	WorkflowManifestSchemaVersion   = 1

	WorkflowPublicationSourceTemplate = "template_enterprise_workflow"
	WorkflowPublicationSourceGlobal   = "legacy_global_workflow"
)

// WorkflowPublicationStep is lossless relative to WorkflowStepDTO and adds the
// stable step_key required by immutable publication manifests.
type WorkflowPublicationStep struct {
	StepKey string `json:"step_key"`
	WorkflowStepDTO
}

// WorkflowPublicationManifestV1 is the canonical workflow snapshot owned by one
// disclosure_type_versions row. Runtime code must read it only from the active
// template version.
type WorkflowPublicationManifestV1 struct {
	SchemaVersion int                       `json:"schema_version"`
	Steps         []WorkflowPublicationStep `json:"steps"`
}

// WorkflowPublicationManifest keeps the existing internal API source-compatible.
type WorkflowPublicationManifest = WorkflowPublicationManifestV1

type templatePublicationBlock struct {
	BlockKey     string         `json:"block_key"`
	BlockType    string         `json:"block_type"`
	Title        string         `json:"title"`
	NameEN       string         `json:"name_en"`
	NameVI       string         `json:"name_vi"`
	Description  string         `json:"description"`
	Config       map[string]any `json:"config"`
	Validation   map[string]any `json:"validation"`
	DisplayOrder int            `json:"display_order"`
	Enabled      bool           `json:"enabled"`
}

// TemplatePublicationCandidate is computed by the app layer before draft
// persistence. Repositories persist these fields in the same transaction as
// the version content and blocks.
type TemplatePublicationCandidate struct {
	AuthorityMode   string
	Manifest        WorkflowPublicationManifest
	ManifestJSON    []byte
	ManifestHash    string
	Source          string
	SourceVersionNo int
	CandidateHash   string
}

// BuildTemplatePublicationCandidate converts the existing enterprise_workflow
// request block into a deterministic, immutable template publication candidate.
func BuildTemplatePublicationCandidate(req UpsertTypeVersionRequest) (TemplatePublicationCandidate, error) {
	steps := ExtractTemplateWorkflow(req.Blocks)
	manifest, manifestJSON, manifestHash, err := CanonicalWorkflowPublication(steps)
	if err != nil {
		return TemplatePublicationCandidate{}, err
	}
	hashInput := struct {
		SchemaVersion         int                         `json:"schema_version"`
		TypeID                string                      `json:"type_id"`
		Scope                 string                      `json:"scope"`
		GroupID               string                      `json:"group_id"`
		Name                  string                      `json:"name"`
		Category              string                      `json:"category"`
		TemplateCategory      string                      `json:"template_category"`
		DeadlineStrategy      string                      `json:"deadline_strategy"`
		Description           string                      `json:"description"`
		LegalBasis            string                      `json:"legal_basis"`
		Applicability         string                      `json:"applicability"`
		ImplementationContent string                      `json:"implementation_content"`
		ImplementationNotes   string                      `json:"implementation_notes"`
		SpecialCases          string                      `json:"special_cases"`
		ReportContent         string                      `json:"report_content"`
		RequiredDocs          string                      `json:"required_docs"`
		DeadlineRule          string                      `json:"deadline_rule"`
		Periodicity           string                      `json:"periodicity"`
		ChannelsText          string                      `json:"channels_text"`
		Beneficiaries         string                      `json:"beneficiaries"`
		ReceivingAuthorities  string                      `json:"receiving_authorities"`
		Format                string                      `json:"format"`
		LegalRisksText        string                      `json:"legal_risks_text"`
		GeneralInfo           string                      `json:"general_info"`
		DeadlineConfig        *TemplateDeadlineConfig     `json:"deadline_config"`
		LegalBases            []LegalBasisDTO             `json:"legal_bases"`
		Checklist             []ChecklistItemDTO          `json:"checklist"`
		Tags                  []string                    `json:"tags"`
		DisplayGroupCodes     []string                    `json:"display_group_codes"`
		ApplicabilityRules    any                         `json:"applicability_rules"`
		Blocks                []templatePublicationBlock  `json:"blocks"`
		Workflow              WorkflowPublicationManifest `json:"workflow"`
	}{
		SchemaVersion: WorkflowManifestSchemaVersion, TypeID: strings.TrimSpace(req.TypeID),
		Scope: strings.TrimSpace(req.Scope), GroupID: strings.TrimSpace(req.GroupID), Name: strings.TrimSpace(req.Name),
		Category: strings.TrimSpace(req.Category), TemplateCategory: strings.TrimSpace(req.TemplateCategory),
		DeadlineStrategy: strings.TrimSpace(req.DeadlineStrategy), Description: req.Description,
		LegalBasis: req.LegalBasis, Applicability: req.Applicability, ImplementationContent: req.ImplementationContent,
		ImplementationNotes: req.ImplementationNotes, SpecialCases: req.SpecialCases, ReportContent: req.ReportContent,
		RequiredDocs: req.RequiredDocs, DeadlineRule: strings.TrimSpace(req.DeadlineRule), Periodicity: strings.TrimSpace(req.Periodicity),
		ChannelsText: req.ChannelsText, Beneficiaries: req.Beneficiaries, ReceivingAuthorities: req.ReceivingAuthorities,
		Format: req.Format, LegalRisksText: req.LegalRisksText, GeneralInfo: req.GeneralInfo,
		DeadlineConfig: req.DeadlineConfig, LegalBases: nonNilLegalBases(req.LegalBases),
		Checklist: nonNilChecklist(req.Checklist), Tags: nonNilStrings(req.Tags),
		DisplayGroupCodes: nonNilStrings(req.DisplayGroupCodes), ApplicabilityRules: req.ApplicabilityRules,
		Blocks: normalizedTemplatePublicationBlocks(req.Blocks),
		Workflow: manifest,
	}
	candidateJSON, err := marshalCanonicalJSON(hashInput)
	if err != nil {
		return TemplatePublicationCandidate{}, fmt.Errorf("marshal publication candidate: %w", err)
	}
	return TemplatePublicationCandidate{
		AuthorityMode: WorkflowAuthorityTemplatePinned, Manifest: manifest, ManifestJSON: manifestJSON,
		ManifestHash: manifestHash, Source: WorkflowPublicationSourceTemplate,
		CandidateHash: sha256Hex(candidateJSON),
	}, nil
}

// BuildLegacyMigrationPublicationCandidate converts the compatibility resolver
// output into the exact template-owned shape without mutating legacy stores.
func BuildLegacyMigrationPublicationCandidate(detail DisclosureTypeDTO, steps []WorkflowStepDTO, source string, sourceVersionNo int) (TemplatePublicationCandidate, error) {
	rawSteps, err := json.Marshal(steps)
	if err != nil {
		return TemplatePublicationCandidate{}, err
	}
	var projected []any
	if err := json.Unmarshal(rawSteps, &projected); err != nil {
		return TemplatePublicationCandidate{}, err
	}
	blocks := make([]TemplateBlockDTO, 0, len(detail.Blocks)+1)
	replaced := false
	for _, block := range detail.Blocks {
		next := block
		if strings.EqualFold(strings.TrimSpace(next.BlockKey), "enterprise_workflow") {
			next.Config = map[string]any{"steps": projected}
			next.Enabled = true
			replaced = true
		}
		blocks = append(blocks, next)
	}
	if !replaced {
		blocks = append(blocks, TemplateBlockDTO{
			BlockID: "enterprise-workflow", BlockKey: "enterprise_workflow", BlockType: "workflow",
			Title: "Workflow", Config: map[string]any{"steps": projected}, Validation: map[string]any{},
			DisplayOrder: len(blocks) + 1, Enabled: true,
		})
	}
	candidate, err := BuildTemplatePublicationCandidate(UpsertTypeVersionRequest{
		TypeID: detail.TypeID, Scope: detail.Scope, GroupID: detail.GroupID, Name: detail.Name,
		Category: detail.Category, TemplateCategory: detail.TemplateCategory, DeadlineStrategy: detail.DeadlineStrategy,
		Description: detail.Description, LegalBasis: detail.LegalBasis, Applicability: detail.Applicability,
		ImplementationContent: detail.ImplementationContent, ImplementationNotes: detail.ImplementationNotes,
		SpecialCases: detail.SpecialCases, ReportContent: detail.ReportContent, RequiredDocs: detail.RequiredDocs,
		DeadlineRule: detail.DeadlineRule, Periodicity: detail.Periodicity, ChannelsText: detail.ChannelsText,
		Beneficiaries: detail.Beneficiaries, ReceivingAuthorities: detail.ReceivingAuthorities, Format: detail.Format,
		LegalRisksText: detail.LegalRisksText, GeneralInfo: detail.GeneralInfo, DeadlineConfig: detail.DeadlineConfig,
		LegalBases: detail.LegalBases, Checklist: detail.Checklist, Tags: detail.Tags, Blocks: blocks,
		DisplayGroupCodes: detail.DisplayGroupCodes, ApplicabilityRules: detail.ApplicabilityRules,
	})
	if err != nil {
		return TemplatePublicationCandidate{}, err
	}
	candidate.Source = source
	candidate.SourceVersionNo = sourceVersionNo
	return candidate, nil
}

func CanonicalWorkflowPublication(steps []WorkflowStepDTO) (WorkflowPublicationManifest, []byte, string, error) {
	cloned := cloneWorkflowStepDTOs(steps)
	out := make([]WorkflowPublicationStep, 0, len(cloned))
	for i, step := range cloned {
		key := strings.TrimSpace(step.StepID)
		if key == "" {
			key = fmt.Sprintf("step-%03d", i+1)
		}
		out = append(out, WorkflowPublicationStep{StepKey: key, WorkflowStepDTO: step})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DisplayOrder != out[j].DisplayOrder {
			return out[i].DisplayOrder < out[j].DisplayOrder
		}
		return out[i].StepKey < out[j].StepKey
	})
	manifest := WorkflowPublicationManifest{SchemaVersion: WorkflowManifestSchemaVersion, Steps: out}
	raw, err := marshalCanonicalJSON(manifest)
	if err != nil {
		return WorkflowPublicationManifest{}, nil, "", fmt.Errorf("marshal workflow publication manifest: %w", err)
	}
	return manifest, raw, sha256Hex(raw), nil
}

func marshalCanonicalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	return encodeCanonicalJSON(generic)
}

func encodeCanonicalJSON(v any) ([]byte, error) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return nil, err
			}
			b.Write(kb)
			b.WriteByte(':')
			vb, err := encodeCanonicalJSON(t[k])
			if err != nil {
				return nil, err
			}
			b.Write(vb)
		}
		b.WriteByte('}')
		return []byte(b.String()), nil
	case []any:
		var b strings.Builder
		b.WriteByte('[')
		for i, item := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			vb, err := encodeCanonicalJSON(item)
			if err != nil {
				return nil, err
			}
			b.Write(vb)
		}
		b.WriteByte(']')
		return []byte(b.String()), nil
	default:
		return json.Marshal(t)
	}
}

// ResolveTemplatePublicationWorkflow is the only normal-runtime resolver for
// CMS default workflow after cutover.
func ResolveTemplatePublicationWorkflow(typeID string, versionNo int, manifest WorkflowPublicationManifest) EffectiveWorkflowDTO {
	steps := make([]WorkflowStepDTO, 0, len(manifest.Steps))
	for _, item := range manifest.Steps {
		steps = append(steps, cloneWorkflowStepDTOs([]WorkflowStepDTO{item.WorkflowStepDTO})[0])
	}
	return EffectiveWorkflowDTO{
		TypeID: typeID, Source: CMSDefaultSourceTemplateEnterprise, VersionNo: versionNo,
		Workflow: steps, HasWorkflow: len(steps) > 0,
	}
}

func ParseWorkflowPublicationManifest(raw []byte) (WorkflowPublicationManifest, error) {
	var manifest WorkflowPublicationManifest
	if len(raw) == 0 {
		return manifest, fmt.Errorf("workflow publication manifest is empty")
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return manifest, fmt.Errorf("decode workflow publication manifest: %w", err)
	}
	if manifest.SchemaVersion != WorkflowManifestSchemaVersion {
		return manifest, fmt.Errorf("unsupported workflow manifest schema_version %d", manifest.SchemaVersion)
	}
	return manifest, nil
}

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return append([]string(nil), in...)
}

func nonNilLegalBases(in []LegalBasisDTO) []LegalBasisDTO {
	if in == nil {
		return []LegalBasisDTO{}
	}
	return append([]LegalBasisDTO(nil), in...)
}

func nonNilChecklist(in []ChecklistItemDTO) []ChecklistItemDTO {
	if in == nil {
		return []ChecklistItemDTO{}
	}
	return append([]ChecklistItemDTO(nil), in...)
}

func normalizedTemplatePublicationBlocks(in []TemplateBlockDTO) []templatePublicationBlock {
	out := make([]templatePublicationBlock, 0, len(in))
	for _, block := range in {
		config := block.Config
		if config == nil {
			config = map[string]any{}
		}
		validation := block.Validation
		if validation == nil {
			validation = map[string]any{}
		}
		out = append(out, templatePublicationBlock{
			BlockKey: strings.TrimSpace(block.BlockKey), BlockType: strings.TrimSpace(block.BlockType),
			Title: block.Title, NameEN: block.NameEN, NameVI: block.NameVI, Description: block.Description,
			Config: config, Validation: validation, DisplayOrder: block.DisplayOrder, Enabled: block.Enabled,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DisplayOrder != out[j].DisplayOrder {
			return out[i].DisplayOrder < out[j].DisplayOrder
		}
		if out[i].BlockKey != out[j].BlockKey {
			return out[i].BlockKey < out[j].BlockKey
		}
		return out[i].BlockType < out[j].BlockType
	})
	return out
}
