// Package getdeck implements the deck read endpoints.
package getdeck

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

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
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeInvalidID, "id must be a positive integer")
			return
		}

		deck, err := store.Deck().GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, edherrors.ErrDeckNotFound) {
				httpUtils.Error(w, http.StatusNotFound, httpUtils.CodeDeckNotFound, "deck not found")
				return
			}
			log.Printf("get deck %d: %v", id, err)
			httpUtils.Error(w, http.StatusInternalServerError, httpUtils.CodeInternal, "internal error")
			return
		}
		httpUtils.JSONEncode(w, http.StatusOK, deck)
	}
}

// ListDecks handles GET /decks.
func ListDecks(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := httpUtils.ParseQueryInt(q.Get("page"), storage.DefaultPage)
		if err != nil || page < 1 {
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeInvalidQuery, "page must be a positive integer")
			return
		}

		limit, err := httpUtils.ParseQueryInt(q.Get("limit"), storage.DefaultLimit)
		if err != nil || limit < 1 || limit > storage.MaxLimit {
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeInvalidQuery,
				fmt.Sprintf("limit must be an integer between 1 and %d", storage.MaxLimit))
			return
		}

		filters := storage.DeckFilters{
			Name:   q.Get("name"),
			Owner:  q.Get("owner"),
			Type:   q.Get("type"),
			Search: strings.TrimSpace(q.Get("search")),
			Page:   page,
			Limit:  limit,
		}

		decks, err := store.Deck().List(r.Context(), filters)
		if err != nil {
			log.Printf("list decks: %v", err)
			httpUtils.Error(w, http.StatusInternalServerError, httpUtils.CodeInternal, "internal error")
			return
		}

		// The size of the whole result, so that a client can say "page 2 of 5" instead
		// of inferring the end from a page that came back short.
		total, err := store.Deck().Count(r.Context(), filters)
		if err != nil {
			log.Printf("count decks: %v", err)
			httpUtils.Error(w, http.StatusInternalServerError, httpUtils.CodeInternal, "internal error")
			return
		}

		w.Header().Set("X-Total-Count", strconv.Itoa(total))
		httpUtils.JSONEncode(w, http.StatusOK, decks)
	}
}
