package sl

import (
	"log/slog"

	"github.com/jackc/pgx"
)

func Err(err error) slog.Attr {
	return slog.Attr{
		Key:   "error",
		Value: slog.StringValue(err.Error()),
	}
}

func PgError(err pgx.PgError) slog.Attr {
	return slog.Attr{
		Key:   "error",
		Value: slog.AnyValue(err),
	}
}

func Module(module string) slog.Attr {
	return slog.Attr{
		Key:   "module",
		Value: slog.StringValue(module),
	}
}

func Method(method string) slog.Attr {
	return slog.Attr{
		Key:   "method",
		Value: slog.StringValue(method),
	}
}
