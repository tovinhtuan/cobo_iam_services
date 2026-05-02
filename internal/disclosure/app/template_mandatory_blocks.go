package app

// MandatoryTemplateBlockKeys is the canonical set required on every template that defines at least one block.
// Templates may include additional blocks; keys must remain unique across the matrix.
//
// Mirrors cobo_web_design templateBlockSchema.TEMPLATE_MANDATORY_BLOCK_KEYS.
var MandatoryTemplateBlockKeys = []string{
	"legal_basis",
	"disclosure_content",
	"deadline",
	"channels_and_format",
	"legal_risks",
	"enterprise_workflow",
}
