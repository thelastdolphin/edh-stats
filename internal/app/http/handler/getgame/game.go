// Package getgame implements the game read endpoints.
package getgame

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

// GetGame handles GET /games/{id}.
func GetGame(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeInvalidID, "id must be a positive integer")
			return
		}

		game, err := store.Game().GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, edherrors.ErrGameNotFound) {
				httpUtils.Error(w, http.StatusNotFound, httpUtils.CodeGameNotFound, "game not found")
				return
			}
			log.Printf("get game %d: %v", id, err)
			httpUtils.Error(w, http.StatusInternalServerError, httpUtils.CodeInternal, "internal error")
			return
		}
		httpUtils.JSONEncode(w, http.StatusOK, game)
	}
}

// ListGames handles GET /games.
//
// The games come back newest first, each with its participants.
func ListGames(store *storage.Store) http.HandlerFunc {
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

		var deckID int64
		if raw := q.Get("deck_id"); raw != "" {
			deckID, err = strconv.ParseInt(raw, 10, 64)
			if err != nil || deckID < 1 {
				httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeInvalidQuery, "deck_id must be a positive integer")
				return
			}
		}

		from, err := httpUtils.ParseQueryDate(q.Get("from"))
		if err != nil {
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeInvalidQuery, "from must be a date in YYYY-MM-DD form")
			return
		}

		to, err := httpUtils.ParseQueryDate(q.Get("to"))
		if err != nil {
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeInvalidQuery, "to must be a date in YYYY-MM-DD form")
			return
		}

		games, err := store.Game().List(r.Context(), storage.GameFilters{
			DeckID: deckID,
			Player: q.Get("player"),
			From:   from,
			To:     to,
			Page:   page,
			Limit:  limit,
		})
		if err != nil {
			log.Printf("list games: %v", err)
			httpUtils.Error(w, http.StatusInternalServerError, httpUtils.CodeInternal, "internal error")
			return
		}
		httpUtils.JSONEncode(w, http.StatusOK, games)
	}
}
