// Package requestid carries the X-Request-ID value through context.
package requestid

import "context"

// Header is the HTTP header that carries the request ID.
const Header = "X-Request-ID"

type ctxKey struct{}

// With returns a copy of ctx carrying id.
func With(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// From returns the request ID stored in ctx, or "".
func From(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}
