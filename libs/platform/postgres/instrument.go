package postgres

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/tehrelt/icyre/libs/platform/postgres"

type queryMetrics struct {
	duration *prometheus.HistogramVec
}

func newQueryMetrics(reg prometheus.Registerer) *queryMetrics {
	m := &queryMetrics{
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "PostgreSQL query latency by SQL operation and result.",
			Buckets: []float64{.0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
		}, []string{"operation", "result"}),
	}
	reg.MustRegister(m.duration)
	return m
}

// tracer implements pgx.QueryTracer: one span and one histogram sample per query.
type tracer struct {
	metrics *queryMetrics
	otel    trace.Tracer
}

func newTracer(m *queryMetrics) *tracer {
	return &tracer{metrics: m, otel: otel.Tracer(tracerName)}
}

type queryCtxKey struct{}

type queryState struct {
	start     time.Time
	operation string
	span      trace.Span
}

func (t *tracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	op := operation(data.SQL)
	ctx, span := t.otel.Start(ctx, "db "+op,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", op),
		),
	)
	return context.WithValue(ctx, queryCtxKey{}, &queryState{start: time.Now(), operation: op, span: span})
}

func (t *tracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	st, ok := ctx.Value(queryCtxKey{}).(*queryState)
	if !ok {
		return
	}
	result := "ok"
	if data.Err != nil && data.Err != pgx.ErrNoRows {
		result = "error"
		st.span.RecordError(data.Err)
		st.span.SetStatus(codes.Error, "query failed")
	}
	st.span.End()
	if t.metrics != nil {
		t.metrics.duration.WithLabelValues(st.operation, result).Observe(time.Since(st.start).Seconds())
	}
}

// operation returns a low-cardinality label: the leading SQL keyword.
func operation(sql string) string {
	s := strings.TrimSpace(sql)
	// Skip leading comments like "-- name: GetTrack".
	for strings.HasPrefix(s, "--") {
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = strings.TrimSpace(s[i+1:])
		} else {
			return "other"
		}
	}
	end := strings.IndexFunc(s, func(r rune) bool { return r == ' ' || r == '\n' || r == '\t' || r == '(' })
	if end < 0 {
		end = len(s)
	}
	switch kw := strings.ToLower(s[:end]); kw {
	case "select", "insert", "update", "delete", "with", "begin", "commit", "rollback", "create", "alter", "drop":
		return kw
	default:
		return "other"
	}
}

// poolCollector exports pgxpool statistics.
type poolCollector struct {
	pool                                   *pgxpool.Pool
	total, idle, acquired, max             *prometheus.Desc
	acquireCount, acquireSeconds, canceled *prometheus.Desc
}

func newPoolCollector(pool *pgxpool.Pool) *poolCollector {
	d := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc("db_pool_"+name, help, nil, nil)
	}
	return &poolCollector{
		pool:           pool,
		total:          d("connections_total", "Connections currently open."),
		idle:           d("connections_idle", "Idle connections."),
		acquired:       d("connections_acquired", "Connections currently in use."),
		max:            d("connections_max", "Maximum pool size."),
		acquireCount:   d("acquires_total", "Successful connection acquisitions."),
		acquireSeconds: d("acquire_seconds_total", "Total time spent waiting to acquire connections."),
		canceled:       d("acquires_canceled_total", "Acquisitions cancelled by context."),
	}
}

func (c *poolCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{c.total, c.idle, c.acquired, c.max, c.acquireCount, c.acquireSeconds, c.canceled} {
		ch <- d
	}
}

func (c *poolCollector) Collect(ch chan<- prometheus.Metric) {
	s := c.pool.Stat()
	ch <- prometheus.MustNewConstMetric(c.total, prometheus.GaugeValue, float64(s.TotalConns()))
	ch <- prometheus.MustNewConstMetric(c.idle, prometheus.GaugeValue, float64(s.IdleConns()))
	ch <- prometheus.MustNewConstMetric(c.acquired, prometheus.GaugeValue, float64(s.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(c.max, prometheus.GaugeValue, float64(s.MaxConns()))
	ch <- prometheus.MustNewConstMetric(c.acquireCount, prometheus.CounterValue, float64(s.AcquireCount()))
	ch <- prometheus.MustNewConstMetric(c.acquireSeconds, prometheus.CounterValue, s.AcquireDuration().Seconds())
	ch <- prometheus.MustNewConstMetric(c.canceled, prometheus.CounterValue, float64(s.CanceledAcquireCount()))
}
