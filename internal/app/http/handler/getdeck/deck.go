package getdeck

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"

	httpUtils "example.com/edh-stats/internal/app/http"
	"example.com/edh-stats/internal/app/storage"
)

func GetDeck(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("GetDeck called with path: %s, method: %s", r.URL.Path, r.Method)
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

		deck, err := store.Deck().GetById(r.Context(), id)
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
}

func ListDecks(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("ListDecks called with path: %s, method: %s", r.URL.Path, r.Method)
		q := r.URL.Query()

		filters := storage.DeckFilters{
			Name:  parseQueryString(q, "name"),
			Owner: parseQueryString(q, "owner"),
			Type:  parseQueryString(q, "type"),
			Page:  parseQueryInt(q, "page", 1),
			Limit: parseQueryInt(q, "limit", 20),
		}

		decks, err := store.Deck().List(r.Context(), filters)
		if err != nil {
			log.Printf("Database error: %v", err)
			httpUtils.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpUtils.JSONEncode(w, http.StatusOK, decks)
	}
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
