package observability

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordRequest_IncrementsCounters(t *testing.T) {
	ensureMetrics()
	before := testutil.ToFloat64(requestsTotal.WithLabelValues("200", "30d"))
	RecordRequest("200", "30d", 50*time.Millisecond, false)
	after := testutil.ToFloat64(requestsTotal.WithLabelValues("200", "30d"))
	if after <= before {
		t.Fatalf("expected counter increment, before=%v after=%v", before, after)
	}
}

func TestRecordRequest_Partial(t *testing.T) {
	ensureMetrics()
	before := testutil.ToFloat64(partialTotal.WithLabelValues("7d"))
	RecordRequest("200", "7d", 10*time.Millisecond, true)
	after := testutil.ToFloat64(partialTotal.WithLabelValues("7d"))
	if after <= before {
		t.Fatal("expected partial counter increment")
	}
}
