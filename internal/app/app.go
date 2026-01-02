package app

import (
	"context"
	"fmt"
	"net/http"

	"example.com/edh-stats/internal/app/storage"
)

type Config struct {
	Dbstring      string
	ServerAddress string
}

type Application struct {
	config  *Config // Временная мера
	storage *storage.Store
	server  *http.Server
}

func New(cfg *Config) (*Application, error) {
	app := &Application{config: cfg}

	store, err := storage.NewStore(context.Background(), cfg.Dbstring)
	if err != nil {
		return nil, fmt.Errorf("couldn't create storage: %w", err)
	}
	app.storage = store

	app.server = &http.Server{Addr: cfg.ServerAddress}

	return app, nil
}

func (app *Application) Start() error {
	return app.server.ListenAndServe()
}

func (app *Application) Shutdown(ctx context.Context) error {
	err := app.server.Shutdown(ctx)
	if err != nil {
		return err
	}
	app.storage.Close()
	return nil
}
