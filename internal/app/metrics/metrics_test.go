package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// newTestMux is the smallest thing that behaves like the real router: one route with a
// path parameter, so the pattern label can be checked.
func newTestMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /decks/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /decks", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	return mux
}

func request(handler http.Handler, method, path string) int {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	return recorder.Code
}

// TestMiddlewareUsesTheRoutePattern is the reason the middleware wraps the mux: the label
// has to be the pattern, not the path.
func TestMiddlewareUsesTheRoutePattern(t *testing.T) {
	m := New()
	handler := m.Middleware(newTestMux())

	request(handler, http.MethodGet, "/decks/1")
	request(handler, http.MethodGet, "/decks/2")
	request(handler, http.MethodGet, "/decks/3")

	got := testutil.ToFloat64(m.requests.WithLabelValues(http.MethodGet, "/decks/{id}", "200"))
	if got != 3 {
		t.Errorf("counter for /decks/{id} = %v, want 3", got)
	}

	// The identifiers must not have become labels of their own.
	if series := testutil.CollectAndCount(m.requests); series != 1 {
		t.Errorf("series = %d, want 1: the path must not be a label", series)
	}
}

func TestMiddlewareCountsEveryOutcome(t *testing.T) {
	m := New()
	handler := m.Middleware(newTestMux())

	request(handler, http.MethodPost, "/decks")

	if got := testutil.ToFloat64(m.requests.WithLabelValues(http.MethodPost, "/decks", "201")); got != 1 {
		t.Errorf("counter for POST /decks = %v, want 1", got)
	}
	if count := testutil.CollectAndCount(m.duration); count == 0 {
		t.Error("no duration was observed")
	}
}

// TestMiddlewareSkipsItself keeps scraping out of the traffic: a metrics endpoint that
// counts its own scrapes makes every rate graph lie a little.
func TestMiddlewareSkipsItself(t *testing.T) {
	m := New()
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", m.Handler())
	handler := m.Middleware(mux)

	request(handler, http.MethodGet, "/metrics")
	request(handler, http.MethodGet, "/metrics")

	if series := testutil.CollectAndCount(m.requests); series != 0 {
		t.Errorf("series = %d, want 0: /metrics must not be counted", series)
	}
}

func TestHandlerServesText(t *testing.T) {
	m := New()

	// A counter vector publishes nothing until it has a label combination, so a fresh
	// registry would be missing the request series this test looks for. Make one request
	// first, which is also how the endpoint behaves in production.
	request(m.Middleware(newTestMux()), http.MethodGet, "/decks/1")

	recorder := httptest.NewRecorder()
	m.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	body := recorder.Body.String()
	for _, fragment := range []string{
		"edh_stats_http_requests_total",
		"edh_stats_http_request_duration_seconds",
		"go_goroutines",
		"process_resident_memory_bytes",
	} {
		if !strings.Contains(body, fragment) {
			t.Errorf("the metrics body does not mention %q", fragment)
		}
	}
}

// TestRequestsThatNeverReachedARoute covers the token rejections and the 404s: they share
// one series instead of exploding into one per path.
func TestRequestsThatNeverReachedARoute(t *testing.T) {
	m := New()
	handler := m.Middleware(newTestMux())

	request(handler, http.MethodGet, "/nonsense")
	request(handler, http.MethodGet, "/nothing-here")

	if got := testutil.ToFloat64(m.requests.WithLabelValues(http.MethodGet, "unmatched", "404")); got != 2 {
		t.Errorf("counter for unmatched requests = %v, want 2", got)
	}
}
