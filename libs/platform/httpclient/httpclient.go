// Package httpclient builds instrumented HTTP clients for service-to-service
// calls: W3C trace propagation, X-Request-ID forwarding and a hard timeout.
package httpclient

import (
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/tehrelt/icyre/libs/platform/requestid"
)

// Config configures a client.
type Config struct {
	// Timeout bounds a whole call, including reading the body.
	Timeout time.Duration
	// MaxIdleConnsPerHost keeps connections to one upstream warm.
	MaxIdleConnsPerHost int
}

// New returns a client whose requests carry the caller's trace context and
// request ID. Use one client per upstream service.
func New(cfg Config) *http.Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Second
	}
	if cfg.MaxIdleConnsPerHost <= 0 {
		cfg.MaxIdleConnsPerHost = 32
	}
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.MaxIdleConnsPerHost = cfg.MaxIdleConnsPerHost

	return &http.Client{
		Timeout:   cfg.Timeout,
		Transport: otelhttp.NewTransport(requestIDTransport{next: base}),
	}
}

// requestIDTransport forwards the request ID found in the request context.
type requestIDTransport struct{ next http.RoundTripper }

func (t requestIDTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if id := requestid.From(r.Context()); id != "" && r.Header.Get(requestid.Header) == "" {
		r = r.Clone(r.Context())
		r.Header.Set(requestid.Header, id)
	}
	return t.next.RoundTrip(r)
}
