// Package http contains the shared HTTP helpers used by the handlers.
package http

import (
	"log"
	"net/http"
	"time"
)

// StatusRecorder captures the status code for the access log.
type StatusRecorder struct {
	http.ResponseWriter
	Status int
}

func (r *StatusRecorder) WriteHeader(status int) {
	r.Status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *StatusRecorder) Write(body []byte) (int, error) {
	if r.Status == 0 {
		r.Status = http.StatusOK
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
		recorder := &StatusRecorder{ResponseWriter: w}

		next.ServeHTTP(recorder, r)

		if recorder.Status == 0 {
			recorder.Status = http.StatusOK
		}
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, recorder.Status,
			time.Since(start).Round(time.Millisecond))
	})
}
