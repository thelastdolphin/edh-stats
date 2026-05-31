package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"

	httpUtils "example.com/edh-stats/internal/app/http"
	"example.com/edh-stats/internal/app/http/handler/create"
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

func parseQueryString(q url.Values, key string) string {
	return q.Get(key)
}

func parseQueryInt(q url.Values, key string, defaultValue int) int {
	if val := q.Get(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultValue
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
	mux.HandleFunc("GET /decks/{id}", app.GetDeck)
	mux.HandleFunc("GET /decks", app.ListDecks)

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

func (app *Application) GetDeck(w http.ResponseWriter, r *http.Request) {
	deckId := r.PathValue("id")

	if len(deckId) <= 0 {
		httpUtils.Error(w, http.StatusBadRequest, "deckId not provided")
		return
	}

	id, err := strconv.Atoi(deckId)
	if err != nil {
		httpUtils.Error(w, http.StatusBadRequest, "couldn't convert deckId to int")
		return
	}

	deck, err := app.storage.Deck().GetById(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpUtils.Error(w, http.StatusNotFound, "deck not found")
			return
		}
		log.Printf("Database error: %v", err)
		httpUtils.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpUtils.JSONEncode(w, http.StatusOK, deck)
}

func (app *Application) ListDecks(w http.ResponseWriter, r *http.Request) {
	log.Printf("ListDecks called with path: %s, method: %s", r.URL.Path, r.Method)
	q := r.URL.Query()

	filters := storage.DeckFilters{
		Name:  parseQueryString(q, "name"),
		Owner: parseQueryString(q, "owner"),
		Type:  parseQueryString(q, "type"),
		Page:  parseQueryInt(q, "page", 1),
		Limit: parseQueryInt(q, "limit", 20),
	}

	decks, err := app.storage.Deck().List(r.Context(), filters)
	if err != nil {
		log.Printf("Database error: %v", err)
		httpUtils.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpUtils.JSONEncode(w, http.StatusOK, decks)
}
