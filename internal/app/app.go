// Package app wires configuration, storage and HTTP routing together.
package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	httpUtils "github.com/thelastdolphin/edh-stats/internal/app/http"
	"github.com/thelastdolphin/edh-stats/internal/app/http/handler/create"
	"github.com/thelastdolphin/edh-stats/internal/app/http/handler/creategame"
	"github.com/thelastdolphin/edh-stats/internal/app/http/handler/deletedeck"
	"github.com/thelastdolphin/edh-stats/internal/app/http/handler/deletegame"
	"github.com/thelastdolphin/edh-stats/internal/app/http/handler/getdeck"
	"github.com/thelastdolphin/edh-stats/internal/app/http/handler/getgame"
	"github.com/thelastdolphin/edh-stats/internal/app/http/handler/getstats"
	"github.com/thelastdolphin/edh-stats/internal/app/http/handler/health"
	"github.com/thelastdolphin/edh-stats/internal/app/http/handler/patchdeck"
	"github.com/thelastdolphin/edh-stats/internal/app/http/handler/patchgame"
	"github.com/thelastdolphin/edh-stats/internal/app/metrics"
	"github.com/thelastdolphin/edh-stats/internal/app/storage"
)

// HTTP timeouts. A zero value in http.Server means "no limit", which lets a
// slow or idle client hold a connection indefinitely.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
)

// Config holds the runtime settings of the service.
type Config struct {
	DbPath        string
	ServerAddress string
	// APIToken, when set, has to be presented as a bearer token on every request
	// except /health. Empty leaves the API open, and the service warns about it.
	APIToken string
}

// Application owns the HTTP server and the storage it serves from.
type Application struct {
	storage *storage.Store
	Server  *http.Server
}

// New prepares storage (applying pending migrations) and the HTTP routes.
func New(ctx context.Context, cfg Config) (*Application, error) {
	store, err := storage.NewStore(ctx, cfg.DbPath)
	if err != nil {
		return nil, fmt.Errorf("couldn't create storage: %w", err)
	}

	// The registry is created here, so the application owns its metrics instead of
	// relying on a process-wide one that tests would share.
	serviceMetrics := metrics.New()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /decks", create.Deck(store))
	mux.HandleFunc("GET /decks", getdeck.ListDecks(store))
	mux.HandleFunc("GET /decks/{id}", getdeck.GetDeck(store))
	mux.HandleFunc("PATCH /decks/{id}", patchdeck.Deck(store))
	mux.HandleFunc("DELETE /decks/{id}", deletedeck.Deck(store))
	mux.HandleFunc("POST /games", creategame.Game(store))
	mux.HandleFunc("GET /games", getgame.ListGames(store))
	mux.HandleFunc("GET /games/{id}", getgame.GetGame(store))
	mux.HandleFunc("DELETE /games/{id}", deletegame.Game(store))
	mux.HandleFunc("PATCH /games/{id}", patchgame.Game(store))
	mux.HandleFunc("PATCH /games/{id}/decks/{deckID}", patchgame.Participant(store))
	mux.HandleFunc("GET /stats/decks", getstats.DeckStats(store))
	mux.HandleFunc("GET /health", health.Health(store))
	// Behind the same token as everything else: a scrape config sends the header.
	mux.Handle("GET /metrics", serviceMetrics.Handler())

	// There is deliberately no catch-all "/" route any more: it answered 200 to
	// every unknown path, which also swallowed wrong methods and typos in URLs.
	// ServeMux itself replies 404 for unknown paths and 405 with an Allow header
	// when the path exists but the method does not.
	server := &http.Server{
		Addr: cfg.ServerAddress,
		// Logging wraps the metrics, and the metrics wrap the token check, so that
		// rejected requests are counted too; the route label comes from the mux, which
		// is why the middleware sits above it.
		Handler: httpUtils.LogRequests(serviceMetrics.Middleware(
			httpUtils.RequireToken(cfg.APIToken, mux))),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	return &Application{storage: store, Server: server}, nil
}

// Start serves HTTP until the server is shut down.
func (app *Application) Start() error {
	// ListenAndServe always returns a non-nil error; ErrServerClosed is the
	// expected outcome of Shutdown rather than a failure.
	if err := app.Server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown stops accepting connections and closes storage.
func (app *Application) Shutdown(ctx context.Context) error {
	return errors.Join(app.Server.Shutdown(ctx), app.storage.Close())
}
