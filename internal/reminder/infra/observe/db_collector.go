package observe

import (
	"context"
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Reminder Reliability Hardening — Observability (mirror of the Batch 2B / Option B
// notification EmailDeliveryCollector). The reminder dispatch loop runs only in the
// worker, which exposes no /metrics endpoint. Rather than add an HTTP listener to the
// worker, the API — which already serves /metrics and shares the same MySQL — exposes
// DB-derived gauges describing the reminder pipeline state. These are the authoritative
// alert source for failed / backlog / stuck-dispatching.
//
// Gauges are computed at SCRAPE TIME inside prometheus.Collector.Collect (pull-driven);
// there is no background poller or scheduler. Every scrape runs the COUNT(*) queries
// under a short context timeout so a slow/unhealthy DB can never stall /metrics.

const defaultReminderCollectorQueryTimeout = 2 * time.Second

// reminderFailedRecentWindow bounds cobo_reminder_failed_recent — the "new permanent
// reminder failures since now-window" signal an alert pages on.
const reminderFailedRecentWindow = 15 * time.Minute

// CountSource is the minimal read surface the collector needs. It is satisfied by the
// *sql.DB-backed implementation below and trivially faked in tests.
type CountSource interface {
	// CountBacklog: reminder_occurrences due for dispatch now (PENDING scheduled_at<=now
	// OR RETRY_SCHEDULED next_retry_at<=now).
	CountBacklog(ctx context.Context, now time.Time) (int, error)
	// CountFailedSince: reminder_occurrences gone FAILED since `since`.
	CountFailedSince(ctx context.Context, since time.Time) (int, error)
	// CountFailedTotal: all-time FAILED reminder_occurrences.
	CountFailedTotal(ctx context.Context) (int, error)
	// CountStuckDispatching: reminder_occurrences stuck in DISPATCHING past olderThan
	// (= worker crashed mid-dispatch). Mirrors the reaper's predicate.
	CountStuckDispatching(ctx context.Context, olderThan time.Time) (int, error)
}

// ReminderObservabilityCollector implements prometheus.Collector, emitting the reminder
// reliability gauges plus a scrape-error counter.
type ReminderObservabilityCollector struct {
	src               CountSource
	visibilityTimeout time.Duration
	recentWindow      time.Duration
	queryTimeout      time.Duration
	now               func() time.Time

	backlogDesc         *prometheus.Desc
	failedRecentDesc    *prometheus.Desc
	failedTotalDesc     *prometheus.Desc
	stuckDispatchDesc   *prometheus.Desc
	scrapeErrors        prometheus.Counter
}

// NewReminderObservabilityCollector builds the collector. visibilityTimeout should match
// the worker reaper's REMINDER_VISIBILITY_TIMEOUT so the stuck gauge and the reaper agree
// on "stuck". A non-positive value falls back to 5m.
func NewReminderObservabilityCollector(src CountSource, visibilityTimeout time.Duration) *ReminderObservabilityCollector {
	if visibilityTimeout <= 0 {
		visibilityTimeout = 5 * time.Minute
	}
	return &ReminderObservabilityCollector{
		src:               src,
		visibilityTimeout: visibilityTimeout,
		recentWindow:      reminderFailedRecentWindow,
		queryTimeout:      defaultReminderCollectorQueryTimeout,
		now:               func() time.Time { return time.Now().UTC() },
		backlogDesc: prometheus.NewDesc(
			"cobo_reminder_backlog",
			"Reminder occurrences due for dispatch (PENDING scheduled_at<=now or RETRY_SCHEDULED next_retry_at<=now)",
			nil, nil),
		failedRecentDesc: prometheus.NewDesc(
			"cobo_reminder_failed_recent",
			"Reminder occurrences marked FAILED within the recent alert window",
			nil, nil),
		failedTotalDesc: prometheus.NewDesc(
			"cobo_reminder_failed_total",
			"Reminder occurrences in FAILED state (all time)",
			nil, nil),
		stuckDispatchDesc: prometheus.NewDesc(
			"cobo_reminder_stuck_dispatching",
			"Reminder occurrences stuck in DISPATCHING past the visibility timeout (worker crashed mid-dispatch)",
			nil, nil),
		scrapeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "cobo",
			Subsystem: "reminder",
			Name:      "metrics_scrape_errors_total",
			Help:      "Count of failed reminder-metrics collector queries (collector health)",
		}),
	}
}

// Describe implements prometheus.Collector.
func (c *ReminderObservabilityCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.backlogDesc
	ch <- c.failedRecentDesc
	ch <- c.failedTotalDesc
	ch <- c.stuckDispatchDesc
	c.scrapeErrors.Describe(ch)
}

// Collect implements prometheus.Collector. Each gauge is emitted only when its query
// succeeds; a failure increments scrape-error and omits that gauge so Prometheus surfaces
// staleness rather than a wrong (e.g. zero) value.
func (c *ReminderObservabilityCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.queryTimeout)
	defer cancel()
	now := c.now()

	if v, err := c.src.CountBacklog(ctx, now); err != nil {
		c.scrapeErrors.Inc()
	} else {
		ch <- prometheus.MustNewConstMetric(c.backlogDesc, prometheus.GaugeValue, float64(v))
	}

	if v, err := c.src.CountFailedSince(ctx, now.Add(-c.recentWindow)); err != nil {
		c.scrapeErrors.Inc()
	} else {
		ch <- prometheus.MustNewConstMetric(c.failedRecentDesc, prometheus.GaugeValue, float64(v))
	}

	if v, err := c.src.CountFailedTotal(ctx); err != nil {
		c.scrapeErrors.Inc()
	} else {
		ch <- prometheus.MustNewConstMetric(c.failedTotalDesc, prometheus.GaugeValue, float64(v))
	}

	if v, err := c.src.CountStuckDispatching(ctx, now.Add(-c.visibilityTimeout)); err != nil {
		c.scrapeErrors.Inc()
	} else {
		ch <- prometheus.MustNewConstMetric(c.stuckDispatchDesc, prometheus.GaugeValue, float64(v))
	}

	c.scrapeErrors.Collect(ch)
}

// dbCountSource is the MySQL-backed CountSource. The queries are index-backed and
// read-only — no migration, no schema change, no history-model change.
type dbCountSource struct {
	db *sql.DB
}

// NewDBCountSource wires the collector to a live *sql.DB.
func NewDBCountSource(db *sql.DB) CountSource {
	return &dbCountSource{db: db}
}

func (s *dbCountSource) CountBacklog(ctx context.Context, now time.Time) (int, error) {
	return s.scalar(ctx,
		`SELECT COUNT(*) FROM reminder_occurrences
		 WHERE (status='PENDING' AND scheduled_at <= ?)
		    OR (status='RETRY_SCHEDULED' AND next_retry_at IS NOT NULL AND next_retry_at <= ?)`,
		now.UTC(), now.UTC())
}

func (s *dbCountSource) CountFailedSince(ctx context.Context, since time.Time) (int, error) {
	return s.scalar(ctx,
		`SELECT COUNT(*) FROM reminder_occurrences WHERE status='FAILED' AND updated_at >= ?`,
		since.UTC())
}

func (s *dbCountSource) CountFailedTotal(ctx context.Context) (int, error) {
	return s.scalar(ctx,
		`SELECT COUNT(*) FROM reminder_occurrences WHERE status='FAILED'`)
}

func (s *dbCountSource) CountStuckDispatching(ctx context.Context, olderThan time.Time) (int, error) {
	return s.scalar(ctx,
		`SELECT COUNT(*) FROM reminder_occurrences WHERE status='DISPATCHING' AND updated_at < ?`,
		olderThan.UTC())
}

func (s *dbCountSource) scalar(ctx context.Context, query string, args ...any) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
