// Package http contains the shared HTTP helpers used by the handlers.
package http

import (
	"log"
	"net/http"
	"time"
)

// statusRecorder captures the status code for the access log.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(body)
}

// LogRequests logs one line per request.
//
// This replaces the per-handler entry logs, which duplicated the same
// information in the handlers that remembered to add them and knew nothing
// about the response status or the time spent.
//
// Wrapping the ResponseWriter hides optional interfaces such as http.Flusher;
// that is acceptable for this JSON API and is why the handlers use none of them.
func LogRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(recorder, r)

		if recorder.status == 0 {
			recorder.status = http.StatusOK
		}
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, recorder.status,
			time.Since(start).Round(time.Millisecond))
	})
}
