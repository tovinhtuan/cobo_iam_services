package observe

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Periodic worker observability (PER-06). The periodic seeder/materializer runs only in the
// worker, which exposes no /metrics; like the reminder collector, the API derives gauges
// from the shared MySQL at scrape time. No background poller.

const defaultPeriodicCollectorQueryTimeout = 2 * time.Second

// PeriodicCountSource is the read surface the collector needs.
type PeriodicCountSource interface {
	// CountCyclesByState: periodic_cycles grouped by materialization_state.
	CountCyclesByState(ctx context.Context) (map[string]int, error)
	// CountStaleClaims: CLAIMED cycles older than olderThan (the reaper's predicate).
	CountStaleClaims(ctx context.Context, olderThan time.Duration) (int, error)
	// HasDBCalendar: a CMS-uploaded holiday calendar exists for year.
	HasDBCalendar(ctx context.Context, year int) (bool, error)
}

// PeriodicObservabilityCollector implements prometheus.Collector.
type PeriodicObservabilityCollector struct {
	src          PeriodicCountSource
	holidayDir   string
	claimLease   time.Duration
	queryTimeout time.Duration
	loc          *time.Location
	now          func() time.Time

	cyclesDesc      *prometheus.Desc
	staleDesc       *prometheus.Desc
	holidayYearDesc *prometheus.Desc
	scrapeErrors    prometheus.Counter
}

// NewPeriodicObservabilityCollector builds the collector. holidayDir is the JSON fallback
// directory (configs/non_trading_days); claimLease must match the worker reaper.
func NewPeriodicObservabilityCollector(src PeriodicCountSource, holidayDir string, claimLease time.Duration) *PeriodicObservabilityCollector {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	return &PeriodicObservabilityCollector{
		src:          src,
		holidayDir:   holidayDir,
		claimLease:   claimLease,
		queryTimeout: defaultPeriodicCollectorQueryTimeout,
		loc:          loc,
		now:          time.Now,
		cyclesDesc: prometheus.NewDesc(
			"cobo_periodic_cycles",
			"Periodic cycles by materialization_state",
			[]string{"state"}, nil),
		staleDesc: prometheus.NewDesc(
			"cobo_periodic_cycles_claimed_stale",
			"Periodic cycles CLAIMED longer than the claim lease (worker stopped mid-item; the reaper requeues them)",
			nil, nil),
		holidayYearDesc: prometheus.NewDesc(
			"cobo_holiday_calendar_year_available",
			"1 when a holiday calendar exists for the HCM year in slot (current|next); source db = CMS upload, file = JSON fallback",
			[]string{"slot", "source"}, nil),
		scrapeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "cobo",
			Subsystem: "periodic",
			Name:      "metrics_scrape_errors_total",
			Help:      "Count of failed periodic-metrics collector queries (collector health)",
		}),
	}
}

// Describe implements prometheus.Collector.
func (c *PeriodicObservabilityCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.cyclesDesc
	ch <- c.staleDesc
	ch <- c.holidayYearDesc
	c.scrapeErrors.Describe(ch)
}

// Collect implements prometheus.Collector. A failed query omits its gauge and increments the
// scrape-error counter, so Prometheus shows staleness instead of a wrong zero.
func (c *PeriodicObservabilityCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.queryTimeout)
	defer cancel()

	if byState, err := c.src.CountCyclesByState(ctx); err != nil {
		c.scrapeErrors.Inc()
	} else {
		for state, n := range byState {
			ch <- prometheus.MustNewConstMetric(c.cyclesDesc, prometheus.GaugeValue, float64(n), state)
		}
	}
	if n, err := c.src.CountStaleClaims(ctx, c.claimLease); err != nil {
		c.scrapeErrors.Inc()
	} else {
		ch <- prometheus.MustNewConstMetric(c.staleDesc, prometheus.GaugeValue, float64(n))
	}
	year := c.now().In(c.loc).Year()
	for label, y := range map[string]int{"current": year, "next": year + 1} {
		if ok, err := c.src.HasDBCalendar(ctx, y); err != nil {
			c.scrapeErrors.Inc()
		} else {
			ch <- prometheus.MustNewConstMetric(c.holidayYearDesc, prometheus.GaugeValue, boolGauge(ok), label, "db")
		}
		_, statErr := os.Stat(filepath.Join(c.holidayDir, fmt.Sprintf("%d.json", y)))
		ch <- prometheus.MustNewConstMetric(c.holidayYearDesc, prometheus.GaugeValue, boolGauge(statErr == nil), label, "file")
	}
	c.scrapeErrors.Collect(ch)
}

func boolGauge(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

type dbPeriodicCountSource struct{ db *sql.DB }

// NewDBPeriodicCountSource wires the collector to MySQL (read-only queries).
func NewDBPeriodicCountSource(db *sql.DB) PeriodicCountSource {
	return &dbPeriodicCountSource{db: db}
}

func (s *dbPeriodicCountSource) CountCyclesByState(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT materialization_state, COUNT(*) FROM periodic_cycles GROUP BY materialization_state`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var state string
		var n int
		if err := rows.Scan(&state, &n); err != nil {
			return nil, err
		}
		out[state] = n
	}
	return out, rows.Err()
}

func (s *dbPeriodicCountSource) CountStaleClaims(ctx context.Context, olderThan time.Duration) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM periodic_cycles
		WHERE record_id IS NULL AND materialization_state = 'CLAIMED'
		  AND materialized_at < NOW(3) - INTERVAL ? SECOND`, int(olderThan/time.Second)).Scan(&n)
	return n, err
}

func (s *dbPeriodicCountSource) HasDBCalendar(ctx context.Context, year int) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM holiday_calendars WHERE calendar_year = ?`, year).Scan(&n)
	return n > 0, err
}
