package authn

import (
	"crypto/subtle"
	"net/http"

	"github.com/tehrelt/icyre/libs/platform/httpserver"
)

// InternalTokenHeader carries the shared secret for internal service-to-service calls.
const InternalTokenHeader = "X-Internal-Token"

// RequireInternalToken validates the X-Internal-Token header in constant time.
// It fails closed (403) when expected is empty, returns 401 when the header is
// missing, and returns 403 when the token does not match.
func RequireInternalToken(expected string) func(http.Handler) http.Handler {
	expectedBytes := []byte(expected)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(expectedBytes) == 0 {
				httpserver.WriteError(w, r, http.StatusForbidden, CodeForbidden, "Internal authentication is not configured", nil)
				return
			}
			got := r.Header.Get(InternalTokenHeader)
			if got == "" {
				httpserver.WriteError(w, r, http.StatusUnauthorized, CodeUnauthenticated, "Internal authentication required", nil)
				return
			}
			if subtle.ConstantTimeCompare([]byte(got), expectedBytes) != 1 {
				httpserver.WriteError(w, r, http.StatusForbidden, CodeForbidden, "Invalid internal token", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
