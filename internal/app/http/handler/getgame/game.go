// Package getgame implements the game read endpoints.
package getgame

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/thelastdolphin/edh-stats/internal/app/edherrors"
	httpUtils "github.com/thelastdolphin/edh-stats/internal/app/http"
	"github.com/thelastdolphin/edh-stats/internal/app/model"
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

		page, err := parseQueryInt(q.Get("page"), storage.DefaultPage)
		if err != nil || page < 1 {
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeInvalidQuery, "page must be a positive integer")
			return
		}

		limit, err := parseQueryInt(q.Get("limit"), storage.DefaultLimit)
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

		from, err := parseQueryDate(q.Get("from"))
		if err != nil {
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeInvalidQuery, "from must be a date in YYYY-MM-DD form")
			return
		}

		to, err := parseQueryDate(q.Get("to"))
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

// parseQueryInt parses an optional integer query parameter, falling back to
// defaultValue when the parameter is absent.
//
// A malformed value is reported to the caller instead of being silently replaced
// by the default: quietly ignoring "limit=abc" hides client bugs.
func parseQueryInt(raw string, defaultValue int) (int, error) {
	if raw == "" {
		return defaultValue, nil
	}
	return strconv.Atoi(raw)
}

// parseQueryDate validates an optional date query parameter and returns it in the
// canonical form, so that a client sending "2026-2-7" is answered with an error
// rather than with an empty result.
func parseQueryDate(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}

	parsed, err := time.Parse(model.DateLayout, raw)
	if err != nil {
		return "", err
	}
	return parsed.Format(model.DateLayout), nil
}
