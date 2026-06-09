package observe

import (
	"context"
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Batch 2B Observability (Option B): the EmailDispatchHandler runs only in the
// worker, which exposes no /metrics endpoint. Rather than add an HTTP listener
// to the worker, the API — which already serves /metrics and shares the same
// MySQL — exposes DB-derived gauges describing the email pipeline's state. These
// are the authoritative alert source for failed_permanent / backlog /
// stale-processing.
//
// The gauges are computed at SCRAPE TIME inside prometheus.Collector.Collect
// (pull-driven) — there is no background poller or scheduler. Every scrape runs
// the COUNT(*) queries under a short context timeout so a slow/unhealthy DB can
// never stall the API's /metrics handler.

const defaultCollectorQueryTimeout = 2 * time.Second

// failedPermanentRecentWindow bounds cobo_email_failed_permanent_recent — the
// "new permanent drops since now-window" signal an alert pages on.
const failedPermanentRecentWindow = 15 * time.Minute

// CountSource is the minimal read surface the collector needs. It is satisfied
// by the *sql.DB-backed implementation below and trivially faked in tests.
type CountSource interface {
	// CountBacklog: email_notifications still in flight (pending|sending|retry).
	CountBacklog(ctx context.Context) (int, error)
	// CountFailedPermanentSince: email_notifications gone permanent since `since`.
	CountFailedPermanentSince(ctx context.Context, since time.Time) (int, error)
	// CountFailedPermanentTotal: all-time permanent failures.
	CountFailedPermanentTotal(ctx context.Context) (int, error)
	// CountStaleProcessing: outbox rows stuck in `processing` past olderThan
	// (= worker crashed / not ticking). Mirrors the reaper's predicate.
	CountStaleProcessing(ctx context.Context, olderThan time.Time) (int, error)
}

// EmailDeliveryCollector implements prometheus.Collector, emitting the Batch 2B
// email observability gauges plus a scrape-error counter.
type EmailDeliveryCollector struct {
	src               CountSource
	visibilityTimeout time.Duration
	recentWindow      time.Duration
	queryTimeout      time.Duration
	now               func() time.Time

	backlogDesc         *prometheus.Desc
	failedRecentDesc    *prometheus.Desc
	failedTotalDesc     *prometheus.Desc
	staleProcessingDesc *prometheus.Desc
	scrapeErrors        prometheus.Counter
}

// NewEmailDeliveryCollector builds the collector. visibilityTimeout should match
// the worker reaper's OUTBOX_VISIBILITY_TIMEOUT so the stale gauge and the
// reaper agree on "stuck". A non-positive value falls back to 5m.
func NewEmailDeliveryCollector(src CountSource, visibilityTimeout time.Duration) *EmailDeliveryCollector {
	if visibilityTimeout <= 0 {
		visibilityTimeout = 5 * time.Minute
	}
	return &EmailDeliveryCollector{
		src:               src,
		visibilityTimeout: visibilityTimeout,
		recentWindow:      failedPermanentRecentWindow,
		queryTimeout:      defaultCollectorQueryTimeout,
		now:               func() time.Time { return time.Now().UTC() },
		backlogDesc: prometheus.NewDesc(
			"cobo_email_backlog",
			"Email notifications still in flight (status pending|sending|retry)",
			nil, nil),
		failedRecentDesc: prometheus.NewDesc(
			"cobo_email_failed_permanent_recent",
			"Email notifications marked failed_permanent within the recent alert window",
			nil, nil),
		failedTotalDesc: prometheus.NewDesc(
			"cobo_email_failed_permanent_total",
			"Email notifications in failed_permanent state (all time)",
			nil, nil),
		staleProcessingDesc: prometheus.NewDesc(
			"cobo_outbox_stale_processing",
			"Outbox events stuck in processing past the visibility timeout (worker stalled)",
			nil, nil),
		scrapeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "cobo",
			Subsystem: "email",
			Name:      "metrics_scrape_errors_total",
			Help:      "Count of failed email-metrics collector queries (collector health)",
		}),
	}
}

// Describe implements prometheus.Collector.
func (c *EmailDeliveryCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.backlogDesc
	ch <- c.failedRecentDesc
	ch <- c.failedTotalDesc
	ch <- c.staleProcessingDesc
	c.scrapeErrors.Describe(ch)
}

// Collect implements prometheus.Collector. Each gauge is emitted only when its
// query succeeds; a failure increments scrape-error and omits that gauge so
// Prometheus surfaces staleness rather than a wrong (e.g. zero) value.
func (c *EmailDeliveryCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.queryTimeout)
	defer cancel()
	now := c.now()

	if v, err := c.src.CountBacklog(ctx); err != nil {
		c.scrapeErrors.Inc()
	} else {
		ch <- prometheus.MustNewConstMetric(c.backlogDesc, prometheus.GaugeValue, float64(v))
	}

	if v, err := c.src.CountFailedPermanentSince(ctx, now.Add(-c.recentWindow)); err != nil {
		c.scrapeErrors.Inc()
	} else {
		ch <- prometheus.MustNewConstMetric(c.failedRecentDesc, prometheus.GaugeValue, float64(v))
	}

	if v, err := c.src.CountFailedPermanentTotal(ctx); err != nil {
		c.scrapeErrors.Inc()
	} else {
		ch <- prometheus.MustNewConstMetric(c.failedTotalDesc, prometheus.GaugeValue, float64(v))
	}

	if v, err := c.src.CountStaleProcessing(ctx, now.Add(-c.visibilityTimeout)); err != nil {
		c.scrapeErrors.Inc()
	} else {
		ch <- prometheus.MustNewConstMetric(c.staleProcessingDesc, prometheus.GaugeValue, float64(v))
	}

	c.scrapeErrors.Collect(ch)
}

// dbCountSource is the MySQL-backed CountSource. The queries are index-backed
// (idx_email_notifications_status_scheduled, idx_outbox_status_available) and
// read-only — no migration, no schema change.
type dbCountSource struct {
	db *sql.DB
}

// NewDBCountSource wires the collector to a live *sql.DB.
func NewDBCountSource(db *sql.DB) CountSource {
	return &dbCountSource{db: db}
}

func (s *dbCountSource) CountBacklog(ctx context.Context) (int, error) {
	return s.scalar(ctx,
		`SELECT COUNT(*) FROM email_notifications WHERE status IN ('pending','sending','retry')`)
}

func (s *dbCountSource) CountFailedPermanentSince(ctx context.Context, since time.Time) (int, error) {
	return s.scalar(ctx,
		`SELECT COUNT(*) FROM email_notifications WHERE status='failed_permanent' AND updated_at >= ?`,
		since)
}

func (s *dbCountSource) CountFailedPermanentTotal(ctx context.Context) (int, error) {
	return s.scalar(ctx,
		`SELECT COUNT(*) FROM email_notifications WHERE status='failed_permanent'`)
}

func (s *dbCountSource) CountStaleProcessing(ctx context.Context, olderThan time.Time) (int, error) {
	return s.scalar(ctx,
		`SELECT COUNT(*) FROM outbox_events WHERE status='processing' AND available_at < ?`,
		olderThan)
}

func (s *dbCountSource) scalar(ctx context.Context, query string, args ...any) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
