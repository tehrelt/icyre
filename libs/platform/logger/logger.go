// Package logger builds structured slog loggers for ICYRE services.
//
// Records logged with a context automatically carry request_id and trace_id.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"

	"github.com/tehrelt/icyre/libs/platform/requestid"
)

// Options configures a logger.
type Options struct {
	Service string
	Version string
	// Level is one of debug, info, warn, error. Defaults to info.
	Level string
	// Format is json (default) or text.
	Format string
	Output io.Writer
}

// New returns a logger that writes structured records with the service name.
func New(opts Options) *slog.Logger {
	out := opts.Output
	if out == nil {
		out = os.Stdout
	}
	hopts := &slog.HandlerOptions{
		Level:       ParseLevel(opts.Level),
		ReplaceAttr: redactAttr,
	}

	var h slog.Handler
	if strings.EqualFold(opts.Format, "text") {
		h = slog.NewTextHandler(out, hopts)
	} else {
		h = slog.NewJSONHandler(out, hopts)
	}

	attrs := []slog.Attr{slog.String("service", opts.Service)}
	if opts.Version != "" {
		attrs = append(attrs, slog.String("version", opts.Version))
	}
	return slog.New(contextHandler{h.WithAttrs(attrs)})
}

// ParseLevel converts a level name into slog.Level. Unknown names map to info.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Err is a shorthand attribute for errors.
func Err(err error) slog.Attr {
	return slog.Any("error", err)
}

// contextHandler enriches records with request and trace identifiers.
type contextHandler struct{ slog.Handler }

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := requestid.From(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		r.AddAttrs(slog.String("trace_id", sc.TraceID().String()))
	}
	return h.Handler.Handle(ctx, r)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
}

const redactedValue = "[REDACTED]"

var sensitiveKeys = map[string]struct{}{
	"password":            {},
	"passwd":              {},
	"secret":              {},
	"secret_key":          {},
	"access_key":          {},
	"private_key":         {},
	"signing_key":         {},
	"api_key":             {},
	"token":               {},
	"access_token":        {},
	"refresh_token":       {},
	"id_token":            {},
	"authorization":       {},
	"cookie":              {},
	"set_cookie":          {},
	"dsn":                 {},
	"database_url":        {},
	"presigned_url":       {},
	"signed_url":          {},
	"clickhouse_password": {},
	"redis_password":      {},
	"s3_secret_key":       {},
	"s3_access_key":       {},
}

var sensitiveSuffixes = []string{
	"_password",
	"_passwd",
	"_secret",
	"_secret_key",
	"_private_key",
	"_signing_key",
	"_access_key",
	"_api_key",
	"_token",
}

func isSensitiveKey(key string) bool {
	norm := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	if _, ok := sensitiveKeys[norm]; ok {
		return true
	}
	for _, suffix := range sensitiveSuffixes {
		if strings.HasSuffix(norm, suffix) {
			return true
		}
	}
	return false
}

var credentialURLSchemes = []string{
	"postgres://",
	"postgresql://",
	"redis://",
	"rediss://",
	"http://",
	"https://",
}

func hasURLCredentials(v string) bool {
	lower := strings.ToLower(v)
	for _, scheme := range credentialURLSchemes {
		if hasSchemeCredentials(v, lower, scheme) {
			return true
		}
	}
	return false
}

func hasSchemeCredentials(v, lower, scheme string) bool {
	for offset := 0; offset < len(lower); {
		idx := strings.Index(lower[offset:], scheme)
		if idx < 0 {
			return false
		}
		start := offset + idx + len(scheme)
		authority := v[start:]
		if end := strings.IndexAny(authority, "/?# \t\r\n"); end >= 0 {
			authority = authority[:end]
		}
		if userinfo, _, ok := strings.Cut(authority, "@"); ok && strings.Contains(userinfo, ":") {
			return true
		}
		offset = start
	}
	return false
}

func hasBearerToken(lower string) bool {
	const prefix = "bearer "
	for offset := 0; offset < len(lower); {
		idx := strings.Index(lower[offset:], prefix)
		if idx < 0 {
			return false
		}
		pos := offset + idx
		if pos == 0 || !isAlphaNum(lower[pos-1]) {
			if strings.TrimLeft(lower[pos+len(prefix):], " \t") != "" {
				return true
			}
		}
		offset = pos + len(prefix)
	}
	return false
}

func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

func isSensitiveValue(v string) bool {
	lower := strings.ToLower(v)
	if hasBearerToken(lower) {
		return true
	}
	if strings.Contains(v, "X-Amz-Signature=") || strings.Contains(v, "X-Amz-Credential=") {
		return true
	}
	return hasURLCredentials(v)
}

func attrStringValue(v slog.Value) (string, bool) {
	switch v.Kind() {
	case slog.KindString:
		return v.String(), true
	case slog.KindAny:
		switch val := v.Any().(type) {
		case error:
			return val.Error(), true
		case fmt.Stringer:
			return val.String(), true
		}
	}
	return "", false
}

func redactAttr(_ []string, a slog.Attr) slog.Attr {
	if isSensitiveKey(a.Key) {
		return slog.String(a.Key, redactedValue)
	}
	if s, ok := attrStringValue(a.Value); ok && isSensitiveValue(s) {
		return slog.String(a.Key, redactedValue)
	}
	return a
}
