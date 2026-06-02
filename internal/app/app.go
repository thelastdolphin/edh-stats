package app

import (
	"context"
	"fmt"
	"net/http"

	"example.com/edh-stats/internal/app/http/handler/create"
	"example.com/edh-stats/internal/app/http/handler/getdeck"
	"example.com/edh-stats/internal/app/storage"
)

type Config struct {
	DbPath        string
	ServerAddress string
}

type Application struct {
	config  *Config // Временная мера
	storage *storage.Store
	Server  *http.Server
}

func New(cfg *Config) (*Application, error) {
	app := &Application{config: cfg}

	store, err := storage.NewStore(context.Background(), cfg.DbPath)
	if err != nil {
		return nil, fmt.Errorf("couldn't create storage: %w", err)
	}
	app.storage = store

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("POST /decks", create.Deck(store))
	mux.HandleFunc("GET /decks/{id}", getdeck.GetDeck(store))
	mux.HandleFunc("GET /decks", getdeck.ListDecks(store))

	app.Server = &http.Server{Addr: cfg.ServerAddress, Handler: mux}

	return app, nil
}

func (app *Application) Start() error {
	return app.Server.ListenAndServe()
}

func (app *Application) Shutdown(ctx context.Context) error {
	err := app.Server.Shutdown(ctx)
	if err != nil {
		return err
	}
	app.storage.Close()
	return nil
}
