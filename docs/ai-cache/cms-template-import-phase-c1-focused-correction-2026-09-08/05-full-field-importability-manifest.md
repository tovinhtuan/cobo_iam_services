# Phase C.1 Evidence — 05 Full Field Importability Manifest

## V1 Full Importability Manifest (34 Business Fields)

| # | Field Name | Input Schema Path | Normalized Path | Upsert Path | Persisted Column | Reload Path | Expected Transform |
|---|---|---|---|---|---|---|---|
| 1 | `type_id` | `type_id` | `norm.TypeID` | `req.TypeID` (target) | `disclosure_types.type_id` | `detail.TypeID` | Exact slug |
| 2 | `name` | `name` | `norm.Name` | `req.Name` (target) | `v.name` | `detail.Name` | Trimmed string |
| 3 | `description` | `description` | `norm.Description` | `req.Description` | `v.description` | `detail.Description` | Trimmed string |
| 4 | `category` | `category` | `norm.Category` | `req.Category` | `v.category` | `detail.Category` | Preserved |
| 5 | `template_category` | `template_category` | `norm.TemplateCategory` | `req.TemplateCategory` | `v.template_category` | `detail.TemplateCategory` | Lowercase enum |
| 6 | `group_id` | `group_id` | `norm.GroupID` | `req.GroupID` | `t.group_id` | `detail.GroupID` | Preserved |
| 7 | `deadline_strategy` | `deadline_strategy` | `norm.DeadlineStrategy` | `req.DeadlineStrategy` | `v.deadline_strategy` | `detail.DeadlineStrategy` | Lowercase enum |
| 8 | `deadline_rule` | `deadline_rule` | `norm.DeadlineRule` | `req.DeadlineRule` | `v.deadline_rule` | `detail.DeadlineRule` | Preserved / block sync |
| 9 | `periodicity` | `periodicity` | `norm.Periodicity` | `req.Periodicity` | `v.periodicity` | `detail.Periodicity` | Lowercase enum |
| 10 | `legal_basis` | `legal_basis` | `norm.LegalBasis` | `req.LegalBasis` | `v.legal_basis` | `detail.LegalBasis` | Projection synced |
| 11 | `applicability` | `applicability` | `norm.Applicability` | `req.Applicability` | `v.applicability` | `detail.Applicability` | Preserved |
| 12 | `implementation_content` | `implementation_content` | `norm.ImplementationContent` | `req.ImplementationContent` | `v.implementation_content` | `detail.ImplementationContent` | Redacted for CMS editor |
| 13 | `implementation_notes` | `implementation_notes` | `norm.ImplementationNotes` | `req.ImplementationNotes` | `v.implementation_notes` | `detail.ImplementationNotes` | Preserved |
| 14 | `special_cases` | `special_cases` | `norm.SpecialCases` | `req.SpecialCases` | `v.special_cases` | `detail.SpecialCases` | Preserved |
| 15 | `report_content` | `report_content` | `norm.ReportContent` | `req.ReportContent` | `v.report_content` | `detail.ReportContent` | Preserved |
| 16 | `required_docs` | `required_docs` | `norm.RequiredDocs` | `req.RequiredDocs` | `v.required_docs` | `detail.RequiredDocs` | Preserved |
| 17 | `channels_text` | `channels_text` | `norm.ChannelsText` | `req.ChannelsText` | `v.channels_text` | `detail.ChannelsText` | Preserved |
| 18 | `beneficiaries` | `beneficiaries` | `norm.Beneficiaries` | `req.Beneficiaries` | `v.beneficiaries` | `detail.Beneficiaries` | Preserved |
| 19 | `receiving_authorities` | `receiving_authorities` | `norm.ReceivingAuthorities` | `req.ReceivingAuthorities` | `v.receiving_authorities` | `detail.ReceivingAuthorities` | Preserved |
| 20 | `format` | `format` | `norm.Format` | `req.Format` | `v.format` | `detail.Format` | Normalized uppercase |
| 21 | `legal_risks_text` | `legal_risks_text` | `norm.LegalRisksText` | `req.LegalRisksText` | `v.legal_risks_text` | `detail.LegalRisksText` | Preserved |
| 22 | `general_info` | `general_info` | `norm.GeneralInfo` | `req.GeneralInfo` | `v.general_info` | `detail.GeneralInfo` | Preserved |
| 23 | `display_group_codes` | `display_group_codes` | `norm.DisplayGroupCodes` | `req.DisplayGroupCodes` | `template_display_groups` | `detail.DisplayGroupCodes` | Deduped list |
| 24 | `tags` | `tags` | `norm.Tags` | `req.Tags` | `v.tags_json` | `detail.Tags` | Trimmed & deduped |
| 25 | `legal_bases` | `legal_bases` | `norm.LegalBases` | `req.LegalBases` | `v.legal_bases_json` | `detail.LegalBases` | Fresh server IDs |
| 26 | `checklist` | `checklist` | `norm.Checklist` | `req.Checklist` | `v.checklist_json` | `detail.Checklist` | Preserved |
| 27 | `deadline_config` | `deadline_config` | `norm.DeadlineConfig` | `req.DeadlineConfig` | `v.deadline_config_json` | `detail.DeadlineConfig` | Full object mapping |
| 28 | `applicability_rules` | `applicability_rules` | `norm.ApplicabilityRules` | `req.ApplicabilityRules` | `v.applicability_rules_json` | `detail.ApplicabilityRules` | Preserved / default |
| 29 | `blocks` | derived | `norm.*` | `req.Blocks` | `disclosure_template_blocks` | `detail.Blocks` | 6 canonical blocks |
| 30 | `workflow.steps` | `workflow.steps` | `norm.Workflow.Steps` | `enterprise_workflow.config.steps` | `v.workflow_manifest_json` | `detail.WorkflowManifest` | Pinned manifest |
| 31 | `step.department_id` | `step.department_id` | `norm.Workflow.Steps[i]` | `req.DepartmentMappings` applied | `manifest.steps[i].department_id` | `manifest.steps[i]` | Target department |
| 32 | `step.sla_days` | `step.processing_days` | `norm.Workflow.Steps[i]` | `manifest.steps[i].processing_days` | `v.workflow_manifest_json` | `manifest.steps[i]` | Integer > 0 |
| 33 | `step.reminders` | `step.reminder_config` | `norm.Workflow.Steps[i]` | `manifest.steps[i].reminder_config` | `v.workflow_manifest_json` | `manifest.steps[i]` | Enabled + offsets |
| 34 | `document_requirements` | `step.documents` | `norm.Workflow.Steps[i].documents` | `manifest.steps[i].documents` | `v.workflow_manifest_json` | `manifest.steps[i].documents` | FileName kept, ID cleared |
