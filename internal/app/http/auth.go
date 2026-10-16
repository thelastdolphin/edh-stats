package http

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// RequireToken rejects requests that do not carry the expected bearer token.
//
// An empty expected token disables the check, which is the default: a deployment
// that has not been given a token yet keeps working, and the service warns about it
// at startup instead of refusing to run.
//
// GET /health is always allowed. The container health check runs this very binary
// against that endpoint from inside the container, so protecting it would leave the
// container reporting itself as unhealthy.
func RequireToken(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		if !carriesToken(r.Header.Get("Authorization"), token) {
			// Telling the client how to authenticate makes the 401 actionable, and
			// the header is what HTTP expects on an unauthorized response.
			w.Header().Set("WWW-Authenticate", `Bearer realm="edh-stats"`)
			Error(w, http.StatusUnauthorized, CodeUnauthorized, "a valid bearer token is required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// carriesToken reports whether an Authorization header carries the expected token.
//
// The scheme is compared case-insensitively, as RFC 7235 requires, and the token
// itself is compared in constant time so that a wrong token cannot be discovered
// one character at a time from response times.
func carriesToken(header, expected string) bool {
	scheme, credentials, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(strings.TrimSpace(credentials)), []byte(expected)) == 1
}
