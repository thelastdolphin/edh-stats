// Command edh-stats serves the EDH statistics HTTP API.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thelastdolphin/edh-stats/internal/app"
)

const (
	defaultDBPath        = "./data/edh_stats.db"
	defaultServerAddress = ":8080"

	// shutdownTimeout bounds the graceful shutdown: in-flight requests get this
	// long to finish before the process exits.
	shutdownTimeout = 15 * time.Second
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("edh-stats: %v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx, app.Config{
		DbPath:        envOr("DB_PATH", defaultDBPath),
		ServerAddress: envOr("SERVER_ADDRESS", defaultServerAddress),
	})
	if err != nil {
		return err
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- application.Start() }()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		// Restore the default signal handling first: a second SIGINT must still
		// kill the process if the graceful shutdown hangs.
		stop()
		log.Printf("shutdown signal received, stopping")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return application.Shutdown(shutdownCtx)
	}
}

// envOr returns the value of the environment variable or a fallback.
func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
