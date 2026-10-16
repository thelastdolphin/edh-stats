package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const testToken = "s3cret-token"

// newGuardedMux builds the smallest thing that looks like the real service: a
// protected endpoint and the health probe.
func newGuardedMux(token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /decks", func(w http.ResponseWriter, _ *http.Request) {
		JSONEncode(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		JSONEncode(w, http.StatusOK, map[string]string{"status": "healthy"})
	})
	return RequireToken(token, mux)
}

func get(t *testing.T, handler http.Handler, target, authorization string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, target, nil)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// TestRequireToken_DisabledWhenEmpty covers the default of a deployment that has no
// token configured: the API stays exactly as open as it was.
func TestRequireToken_DisabledWhenEmpty(t *testing.T) {
	recorder := get(t, newGuardedMux(""), "/decks", "")

	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestRequireToken(t *testing.T) {
	handler := newGuardedMux(testToken)

	for name, tc := range map[string]struct {
		authorization string
		want          int
	}{
		"no header":                  {"", http.StatusUnauthorized},
		"scheme without credentials": {"Bearer", http.StatusUnauthorized},
		"empty credentials":          {"Bearer ", http.StatusUnauthorized},
		"wrong token":                {"Bearer nope", http.StatusUnauthorized},
		"the token without a scheme": {testToken, http.StatusUnauthorized},
		"another scheme":             {"Basic " + testToken, http.StatusUnauthorized},
		"the token":                  {"Bearer " + testToken, http.StatusOK},
		"scheme in lower case":       {"bearer " + testToken, http.StatusOK},
		"scheme in upper case":       {"BEARER " + testToken, http.StatusOK},
		"surrounding white space":    {"Bearer   " + testToken + "  ", http.StatusOK},
	} {
		t.Run(name, func(t *testing.T) {
			recorder := get(t, handler, "/decks", tc.authorization)

			if recorder.Code != tc.want {
				t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, tc.want, recorder.Body)
			}
			if tc.want != http.StatusUnauthorized {
				return
			}

			if got := recorder.Header().Get("WWW-Authenticate"); got == "" {
				t.Error("a 401 without WWW-Authenticate does not tell the client how to authenticate")
			}
			if got := decodeError(t, recorder).Code; got != CodeUnauthorized {
				t.Errorf("code = %q, want %q", got, CodeUnauthorized)
			}
		})
	}
}

// TestRequireToken_HealthStaysOpen is what keeps the container health check working:
// the probe runs inside the container and carries no token.
func TestRequireToken_HealthStaysOpen(t *testing.T) {
	recorder := get(t, newGuardedMux(testToken), "/health", "")

	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}
