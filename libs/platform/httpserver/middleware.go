package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/tehrelt/icyre/libs/platform/requestid"
)

// Middleware wraps an http.Handler.
type Middleware func(http.Handler) http.Handler

// Chain applies middleware so that the first one is the outermost.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// RequestID reuses a well-formed incoming X-Request-ID or generates one,
// stores it in the context and echoes it in the response.
func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(requestid.Header)
			if !validRequestID.MatchString(id) {
				id = uuid.NewString()
			}
			w.Header().Set(requestid.Header, id)
			next.ServeHTTP(w, r.WithContext(requestid.With(r.Context(), id)))
		})
	}
}

// Recover converts panics into a 500 error response and logs the stack.
func Recover(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					if v == http.ErrAbortHandler {
						panic(v)
					}
					log.ErrorContext(r.Context(), "panic recovered",
						"panic", v, "stack", string(debug.Stack()))
					WriteError(w, r, http.StatusInternalServerError, CodeInternal, "Internal server error", nil)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Timeout bounds the request context. Handlers must honour ctx.Done().
func Timeout(d time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		if d <= 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AccessLog writes one structured record per request.
func AccessLog(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := wrap(w)
			next.ServeHTTP(rec, r)

			level := slog.LevelInfo
			if rec.status >= 500 {
				level = slog.LevelError
			}
			log.LogAttrs(r.Context(), level, "http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Int("bytes", rec.bytes),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}
}

// Tracing starts a server span per request and extracts W3C trace context.
// Probe and scrape endpoints are not traced.
func Tracing(service string) Middleware {
	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, service,
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				return r.Method
			}),
			otelhttp.WithFilter(func(r *http.Request) bool {
				return r.URL.Path != "/metrics" && !strings.HasPrefix(r.URL.Path, "/health/")
			}),
		)
	}
}

// HTTPMetrics holds the request metrics of a service.
type HTTPMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inflight prometheus.Gauge
}

// NewHTTPMetrics registers http_requests_total, http_request_duration_seconds
// and http_requests_in_flight. Labels are low-cardinality only.
func NewHTTPMetrics(reg prometheus.Registerer) *HTTPMetrics {
	m := &HTTPMetrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "HTTP requests by method, route pattern and status code.",
		}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency by method and route pattern.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
		inflight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "HTTP requests currently being served.",
		}),
	}
	reg.MustRegister(m.requests, m.duration, m.inflight)
	return m
}

// Routes must wrap an *http.ServeMux directly: it reads the matched route
// pattern (r.Pattern) after the mux has routed the request, records metrics
// with it and renames the active span.
func (m *HTTPMetrics) Routes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		m.inflight.Inc()
		defer m.inflight.Dec()

		rec := wrap(w)
		next.ServeHTTP(rec, r)

		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		m.requests.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Inc()
		m.duration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())

		if span := trace.SpanFromContext(r.Context()); span.IsRecording() && r.Pattern != "" {
			span.SetName(r.Pattern)
			span.SetAttributes(attribute.String("http.route", r.Pattern))
		}
	})
}

// statusRecorder captures status code and body size.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func wrap(w http.ResponseWriter) *statusRecorder {
	if rec, ok := w.(*statusRecorder); ok {
		return rec
	}
	return &statusRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status = code
		s.wroteHeader = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.wroteHeader = true
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// StandardOptions configures the default middleware stack.
type StandardOptions struct {
	Service        string
	Log            *slog.Logger
	Metrics        *HTTPMetrics
	RequestTimeout time.Duration
}

// Standard wraps mux with the default ICYRE middleware stack:
// request ID → tracing → access log → recovery → timeout → route metrics.
func Standard(mux *http.ServeMux, o StandardOptions) http.Handler {
	var inner http.Handler = mux
	if o.Metrics != nil {
		inner = o.Metrics.Routes(mux)
	}
	return Chain(inner,
		RequestID(),
		Tracing(o.Service),
		AccessLog(o.Log),
		Recover(o.Log),
		Timeout(o.RequestTimeout),
	)
}
