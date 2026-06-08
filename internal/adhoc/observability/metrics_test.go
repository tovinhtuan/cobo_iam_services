package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

const transitionTotalFQName = "cobo_adhoc_proposal_transition_total"

// TestNewPromMetrics_RegistersCounterWithExpectedNameAndLabels covers the
// Batch 5(a) PromMetrics registration contract: NewPromMetrics must register
// cobo_adhoc_proposal_transition_total (Namespace=cobo, Subsystem=adhoc,
// Name=proposal_transition_total) into the default registry, and every series
// it emits must carry exactly the labels company_id, from_status, to_status —
// nothing else (AK.3 cardinality bound; proposal_id/membership_id/actor_id/
// user_id are explicitly forbidden as label values).
func TestNewPromMetrics_RegistersCounterWithExpectedNameAndLabels(t *testing.T) {
	m := NewPromMetrics()
	if m == nil {
		t.Fatal("NewPromMetrics() returned nil")
	}

	m.RecordTransition("company-001", "pending_admin_approval", "approved")

	count, err := testutil.GatherAndCount(prometheus.DefaultGatherer, transitionTotalFQName)
	if err != nil {
		t.Fatalf("GatherAndCount(%s) error = %v", transitionTotalFQName, err)
	}
	if count == 0 {
		t.Fatalf("expected %s to be registered and report at least one series after RecordTransition, got 0", transitionTotalFQName)
	}

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	var family *dto.MetricFamily
	for _, mf := range mfs {
		if mf.GetName() == transitionTotalFQName {
			family = mf
			break
		}
	}
	if family == nil {
		t.Fatalf("metric family %s not found in gathered output", transitionTotalFQName)
	}

	wantLabels := map[string]bool{"company_id": true, "from_status": true, "to_status": true}
	for _, metric := range family.GetMetric() {
		if len(metric.GetLabel()) != len(wantLabels) {
			t.Fatalf("series %v: label count = %d, want exactly %d (%v)", metric.GetLabel(), len(metric.GetLabel()), len(wantLabels), wantLabels)
		}
		seen := make(map[string]bool, len(metric.GetLabel()))
		for _, lp := range metric.GetLabel() {
			name := lp.GetName()
			if !wantLabels[name] {
				t.Fatalf("unexpected label %q present on %s (forbidden — only company_id/from_status/to_status allowed per AK.3)", name, transitionTotalFQName)
			}
			seen[name] = true
		}
		for name := range wantLabels {
			if !seen[name] {
				t.Fatalf("expected label %q missing from series %v", name, metric.GetLabel())
			}
		}
	}
}

// TestNewPromMetrics_IsSafeForRepeatedConstruction covers the sync.Once guard:
// constructing PromMetrics more than once must not panic via duplicate
// prometheus.MustRegister of the same collector — a real risk if wiring code
// (or tests) ever calls NewPromMetrics() more than once within one process.
func TestNewPromMetrics_IsSafeForRepeatedConstruction(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewPromMetrics() must be safe to call repeatedly (sync.Once-guarded registration), but panicked: %v", r)
		}
	}()
	first := NewPromMetrics()
	second := NewPromMetrics()
	if first == nil || second == nil {
		t.Fatal("NewPromMetrics() returned nil on repeated construction")
	}
}

// TestPromMetrics_RecordTransition_IncrementsCounterByLabelCombination covers
// the emission/value contract: RecordTransition must increment the counter
// series identified by the exact (company_id, from_status, to_status) tuple —
// distinct tuples must accumulate independently, and repeated calls with the
// same tuple must accumulate on the same series (Counter semantics).
func TestPromMetrics_RecordTransition_IncrementsCounterByLabelCombination(t *testing.T) {
	m := NewPromMetrics()

	before := counterValue(t, "company-cardinality-001", "approved", "rejected")
	m.RecordTransition("company-cardinality-001", "approved", "rejected")
	m.RecordTransition("company-cardinality-001", "approved", "rejected")
	m.RecordTransition("company-cardinality-001", "pending_admin_approval", "approved")

	after := counterValue(t, "company-cardinality-001", "approved", "rejected")
	if after-before != 2 {
		t.Fatalf("counter{company_id=company-cardinality-001,from_status=approved,to_status=rejected} increased by %v, want 2", after-before)
	}
}

func counterValue(t *testing.T, companyID, fromStatus, toStatus string) float64 {
	t.Helper()
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != transitionTotalFQName {
			continue
		}
		for _, metric := range mf.GetMetric() {
			labels := make(map[string]string, len(metric.GetLabel()))
			for _, lp := range metric.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}
			if labels["company_id"] == companyID && labels["from_status"] == fromStatus && labels["to_status"] == toStatus {
				return metric.GetCounter().GetValue()
			}
		}
	}
	return 0
}
