// Package metrics exposes the service's Prometheus metrics.
//
// The registry belongs to this package instead of being the global default one, so a
// fresh set can be built for every test and two registries never collide.
package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	httpUtils "github.com/thelastdolphin/edh-stats/internal/app/http"
)

// Metrics owns the registry and the collectors built on it.
type Metrics struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// New builds a registry with the HTTP metrics and the runtime ones.
func New() *Metrics {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	m := &Metrics{
		registry: registry,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "edh_stats_http_requests_total",
			Help: "HTTP requests handled, by method, route and status code.",
		}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "edh_stats_http_request_duration_seconds",
			Help: "How long HTTP requests take, by method and route.",
			// The service answers from a local SQLite file, mostly in microseconds: the
			// default buckets start at 5ms and would put nearly everything in the first
			// one, which makes a latency graph useless.
			Buckets: []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
		}, []string{"method", "route"}),
	}
	registry.MustRegister(m.requests, m.duration)

	return m
}

// Handler serves the metrics in the Prometheus text format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// Middleware counts every request and how long it took.
//
// It wraps the mux, so the route label is the pattern the request matched - "/decks/{id}"
// rather than the path. A path carries identifiers, and one series per identifier is how
// a metrics endpoint takes a Prometheus server down. Requests to /metrics itself are left
// out, so that scraping does not look like traffic.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		recorder := &httpUtils.StatusRecorder{ResponseWriter: w, Status: http.StatusOK}
		start := time.Now()

		next.ServeHTTP(recorder, r)

		// Pattern is the pattern the request matched, method included
		// ("GET /decks/{id}"): the method already has a label of its own, so it is
		// stripped here to keep one dimension per fact. A request that never matched a
		// pattern - a typo, or a rejection by the token check - has no route of its own,
		// and all of them share the single "unmatched" series.
		route := strings.TrimPrefix(r.Pattern, r.Method+" ")
		if route == "" {
			route = "unmatched"
		}

		m.requests.WithLabelValues(r.Method, route, strconv.Itoa(recorder.Status)).Inc()
		m.duration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}
