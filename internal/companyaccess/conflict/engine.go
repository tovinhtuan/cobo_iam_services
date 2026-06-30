package conflict

// Engine runs registered detectors against a snapshot (read-only).
type Engine struct {
	registry []Detector
}

func NewEngine(registry []Detector) *Engine {
	cp := make([]Detector, len(registry))
	copy(cp, registry)
	return &Engine{registry: cp}
}

func DefaultEngine() *Engine {
	return NewEngine(DefaultRegistry())
}

// Evaluate runs all detectors and returns merged, sorted results.
func (e *Engine) Evaluate(input EvaluationInput, snapshot *ConfigurationSnapshot) EvaluationOutput {
	if snapshot == nil {
		snapshot = &ConfigurationSnapshot{CompanyID: input.CompanyID, EvaluatedAt: input.EvaluatedAt}
	}
	var all []Result
	for _, d := range e.registry {
		found := d.Detect(snapshot)
		all = append(all, found...)
	}
	merged := MergeAndSort(all)
	return EvaluationOutput{
		CompanyID:     input.CompanyID,
		EvaluatedAt:   input.EvaluatedAt,
		Results:       merged,
		RulesExecuted: len(e.registry),
		RulesSkipped:  0,
		Partial:       false,
	}
}
