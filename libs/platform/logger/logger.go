// Package logger builds structured slog loggers for ICYRE services.
//
// Records logged with a context automatically carry request_id and trace_id.
package logger

import (
	"context"
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
	hopts := &slog.HandlerOptions{Level: ParseLevel(opts.Level)}

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
