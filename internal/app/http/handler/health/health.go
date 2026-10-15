// Package health exposes the service health endpoint.
package health

import (
	"context"
	"log"
	"net/http"
	"time"

	httpUtils "github.com/thelastdolphin/edh-stats/internal/app/http"
	"github.com/thelastdolphin/edh-stats/internal/app/storage"
)

// pingTimeout bounds the database probe so a stuck database cannot keep the
// health check hanging.
const pingTimeout = 500 * time.Millisecond

// Health reports whether the service can reach its database.
//
// The failure detail stays in the log: the response only carries the status, so
// internal error text is not exposed to whoever can reach the endpoint.
func Health(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
		defer cancel()

		if err := store.Ping(ctx); err != nil {
			log.Printf("health check failed: %v", err)
			httpUtils.JSONEncode(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy"})
			return
		}
		httpUtils.JSONEncode(w, http.StatusOK, map[string]string{"status": "healthy"})
	}
}
