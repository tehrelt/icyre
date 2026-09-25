package ports

import "context"

// Library answers which tracks the current listener has saved. It acts on
// behalf of the user whose token travels in the context.
type Library interface {
	SavedTracks(ctx context.Context, trackIDs []string) (map[string]bool, error)
}

type userTokenKey struct{}

// WithUserToken carries the caller's access token so upstream calls made
// for this request act as that user (BFF user context propagation).
func WithUserToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, userTokenKey{}, token)
}

// UserToken returns the caller's access token, or "" for an anonymous request.
func UserToken(ctx context.Context) string {
	t, _ := ctx.Value(userTokenKey{}).(string)
	return t
}
