// Package getdeck implements the deck read endpoints.
package getdeck

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/thelastdolphin/edh-stats/internal/app/edherrors"
	httpUtils "github.com/thelastdolphin/edh-stats/internal/app/http"
	"github.com/thelastdolphin/edh-stats/internal/app/storage"
)

// GetDeck handles GET /decks/{id}.
func GetDeck(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parsed as int64 to match model.Deck.ID; casting through int would
		// truncate the value on 32-bit platforms.
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			// Zero and negative IDs can never exist, so they are bad input
			// rather than a reason to go to the database.
			httpUtils.Error(w, http.StatusBadRequest, "id must be a positive integer")
			return
		}

		deck, err := store.Deck().GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, edherrors.ErrDeckNotFound) {
				httpUtils.Error(w, http.StatusNotFound, "deck not found")
				return
			}
			log.Printf("get deck %d: %v", id, err)
			httpUtils.Error(w, http.StatusInternalServerError, "internal error")
			return
		}
		httpUtils.JSONEncode(w, http.StatusOK, deck)
	}
}

// ListDecks handles GET /decks.
func ListDecks(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parseQueryInt(q.Get("page"), storage.DefaultPage)
		if err != nil || page < 1 {
			httpUtils.Error(w, http.StatusBadRequest, "page must be a positive integer")
			return
		}

		limit, err := parseQueryInt(q.Get("limit"), storage.DefaultLimit)
		if err != nil || limit < 1 || limit > storage.MaxLimit {
			httpUtils.Error(w, http.StatusBadRequest,
				fmt.Sprintf("limit must be an integer between 1 and %d", storage.MaxLimit))
			return
		}

		decks, err := store.Deck().List(r.Context(), storage.DeckFilters{
			Name:  q.Get("name"),
			Owner: q.Get("owner"),
			Type:  q.Get("type"),
			Page:  page,
			Limit: limit,
		})
		if err != nil {
			log.Printf("list decks: %v", err)
			httpUtils.Error(w, http.StatusInternalServerError, "internal error")
			return
		}
		httpUtils.JSONEncode(w, http.StatusOK, decks)
	}
}

// parseQueryInt parses an optional integer query parameter, falling back to
// defaultValue when the parameter is absent.
//
// A malformed value is reported to the caller instead of being silently
// replaced by the default: quietly ignoring "limit=abc" hides client bugs.
func parseQueryInt(raw string, defaultValue int) (int, error) {
	if raw == "" {
		return defaultValue, nil
	}
	return strconv.Atoi(raw)
}
