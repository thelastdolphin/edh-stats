package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func newTestApp(t *testing.T) *Application {
	t.Helper()

	application, err := New(context.Background(), Config{
		DbPath:        filepath.Join(t.TempDir(), "test.db"),
		ServerAddress: "127.0.0.1:0",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		if err := application.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown: %v", err)
		}
	})
	return application
}

func do(t *testing.T, application *Application, method, target string) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	application.Server.Handler.ServeHTTP(recorder, httptest.NewRequest(method, target, nil))
	return recorder
}

// TestRoutes_UnknownPathIs404 is the regression test for the catch-all "/"
// handler, which answered 200 to every unknown path.
func TestRoutes_UnknownPathIs404(t *testing.T) {
	application := newTestApp(t)

	for _, path := range []string{"/", "/whatever", "/decks/1/extra", "/decks/"} {
		recorder := do(t, application, http.MethodGet, path)
		if recorder.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want %d", path, recorder.Code, http.StatusNotFound)
		}
	}
}

// TestRoutes_WrongMethodIs405 checks that ServeMux rejects the method instead of
// letting the request fall through to a permissive handler.
func TestRoutes_WrongMethodIs405(t *testing.T) {
	application := newTestApp(t)

	recorder := do(t, application, http.MethodDelete, "/decks")
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE /decks = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
	if allow := recorder.Header().Get("Allow"); allow == "" {
		t.Error("405 response has no Allow header")
	}
}

func TestRoutes_Health(t *testing.T) {
	application := newTestApp(t)

	recorder := do(t, application, http.MethodGet, "/health")
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /health = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `"healthy"`) {
		t.Errorf("health body = %s, want it to report healthy", recorder.Body)
	}
}

// TestRoutes_InternalErrorsDoNotLeakSQL makes sure a database failure is logged
// rather than returned to the client.
func TestRoutes_InternalErrorsDoNotLeakSQL(t *testing.T) {
	application := newTestApp(t)

	// Closing the database makes every query fail with a driver error.
	if err := application.storage.Close(); err != nil {
		t.Fatalf("close storage: %v", err)
	}

	recorder := do(t, application, http.MethodGet, "/decks")
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("GET /decks = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}

	body := recorder.Body.String()
	for _, leak := range []string{"sql:", "sqlite", "SELECT", "database"} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(leak)) {
			t.Errorf("response body leaks internals (%q): %s", leak, body)
		}
	}
}
