// Command edh-stats serves the EDH statistics HTTP API.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
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

	// healthcheckCommand is the subcommand the container health check runs.
	healthcheckCommand = "healthcheck"

	// healthcheckTimeout keeps the probe short, so Docker can mark the container
	// unhealthy instead of waiting for a hung request.
	healthcheckTimeout = 3 * time.Second
)

func main() {
	// The container health check reuses this very binary instead of an HTTP
	// client installed in the image: no extra package, and no RUN step that would
	// require emulation when the image is built for another architecture.
	if len(os.Args) > 1 && os.Args[1] == healthcheckCommand {
		if err := healthcheck(); err != nil {
			log.Printf("healthcheck: %v", err)
			os.Exit(1)
		}
		return
	}

	if err := run(); err != nil {
		log.Fatalf("edh-stats: %v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	apiToken := os.Getenv("API_TOKEN")
	if apiToken == "" {
		log.Printf("warning: API_TOKEN is not set, so anyone who can reach the service may read and change the data")
	}

	application, err := app.New(ctx, app.Config{
		DbPath:        envOr("DB_PATH", defaultDBPath),
		ServerAddress: envOr("SERVER_ADDRESS", defaultServerAddress),
		APIToken:      apiToken,
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

// healthcheck calls the service's own /health endpoint.
//
// The response body is inspected, not just the status code: a 200 that does not
// report a healthy status would not mean the database is reachable.
func healthcheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), healthcheckTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, healthcheckURL(), nil)
	if err != nil {
		return err
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<10))
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	if !strings.Contains(string(body), `"healthy"`) {
		return fmt.Errorf("unexpected response: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

// healthcheckURL builds the probe URL from the same SERVER_ADDRESS the server
// binds to, replacing an empty or wildcard host with the loopback address.
func healthcheckURL() string {
	address := envOr("SERVER_ADDRESS", defaultServerAddress)

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		// A bare port, such as "8080".
		host, port = "", strings.TrimPrefix(address, ":")
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/health"
}

// envOr returns the value of the environment variable or a fallback.
func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
