// Package patchgame implements PATCH /games/{id} and PATCH /games/{id}/decks/{deckID}.
package patchgame

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/thelastdolphin/edh-stats/internal/app/edherrors"
	httpUtils "github.com/thelastdolphin/edh-stats/internal/app/http"
	"github.com/thelastdolphin/edh-stats/internal/app/http/dto"
	"github.com/thelastdolphin/edh-stats/internal/app/model"
	"github.com/thelastdolphin/edh-stats/internal/app/storage"
)

// Game handles PATCH /games/{id}: the date and the notes of the game itself.
//
// Everything else about a game belongs to a participation, which has its own
// endpoint below, so that fixing one result cannot wipe the other players.
func Game(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseGameID(w, r)
		if !ok {
			return
		}

		var req dto.PatchGameRequest
		if !httpUtils.DecodeJSON(w, r, &req) {
			return
		}

		req.Normalize()
		if err := req.Validate(); err != nil {
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeValidationFailed, err.Error())
			return
		}

		// The stored game carries the values of the fields the request does not
		// mention, and an unknown game is answered here rather than by the update.
		game, err := store.Game().GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, edherrors.ErrGameNotFound) {
				httpUtils.Error(w, http.StatusNotFound, httpUtils.CodeGameNotFound, "game not found")
				return
			}
			log.Printf("read game %d: %v", id, err)
			httpUtils.Error(w, http.StatusInternalServerError, httpUtils.CodeInternal, "internal error")
			return
		}

		req.Apply(game)

		if err := store.Game().Update(r.Context(), game); err != nil {
			if errors.Is(err, edherrors.ErrGameNotFound) {
				httpUtils.Error(w, http.StatusNotFound, httpUtils.CodeGameNotFound, "game not found")
				return
			}
			log.Printf("update game %d: %v", id, err)
			httpUtils.Error(w, http.StatusInternalServerError, httpUtils.CodeInternal, "internal error")
			return
		}

		httpUtils.JSONEncode(w, http.StatusOK, game)
	}
}

// Participant handles PATCH /games/{id}/decks/{deckID}: who piloted the deck in that
// game, and with what result.
func Participant(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gameID, ok := parseGameID(w, r)
		if !ok {
			return
		}

		deckID, err := strconv.ParseInt(r.PathValue("deckID"), 10, 64)
		if err != nil || deckID < 1 {
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeInvalidID, "deckID must be a positive integer")
			return
		}

		var req dto.PatchGameDeckRequest
		if !httpUtils.DecodeJSON(w, r, &req) {
			return
		}

		req.Normalize()
		if err := req.Validate(); err != nil {
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeValidationFailed, err.Error())
			return
		}

		// The stored participation carries the values the request does not mention.
		game, err := store.Game().GetByID(r.Context(), gameID)
		if err != nil {
			if errors.Is(err, edherrors.ErrGameNotFound) {
				httpUtils.Error(w, http.StatusNotFound, httpUtils.CodeGameNotFound, "game not found")
				return
			}
			log.Printf("read game %d: %v", gameID, err)
			httpUtils.Error(w, http.StatusInternalServerError, httpUtils.CodeInternal, "internal error")
			return
		}

		participant := findParticipant(game, deckID)
		if participant == nil {
			httpUtils.Error(w, http.StatusNotFound, httpUtils.CodeParticipantNotFound,
				"the deck does not take part in the game")
			return
		}

		player := participant.Player
		if req.Player != nil {
			player = *req.Player
		}

		resultTypeID := participant.ResultTypeID
		if req.Result != nil {
			resultTypeID, err = resultTypeIDFor(r, store, *req.Result)
			if err != nil {
				log.Printf("update deck %d in game %d: %v", deckID, gameID, err)
				httpUtils.Error(w, http.StatusInternalServerError, httpUtils.CodeInternal, "internal error")
				return
			}
		}

		if err := store.Game().UpdateParticipant(r.Context(), gameID, deckID, player, resultTypeID); err != nil {
			if !errors.Is(err, edherrors.ErrParticipantNotFound) {
				log.Printf("update deck %d in game %d: %v", deckID, gameID, err)
				httpUtils.Error(w, http.StatusInternalServerError, httpUtils.CodeInternal, "internal error")
				return
			}
			httpUtils.Error(w, http.StatusNotFound, httpUtils.CodeParticipantNotFound,
				"the deck does not take part in the game")
			return
		}

		// Answer with the game as it now stands, so a client does not have to guess
		// what the participation looks like after the change.
		updated, err := store.Game().GetByID(r.Context(), gameID)
		if err != nil {
			log.Printf("read game %d after the update: %v", gameID, err)
			httpUtils.Error(w, http.StatusInternalServerError, httpUtils.CodeInternal, "internal error")
			return
		}
		httpUtils.JSONEncode(w, http.StatusOK, updated)
	}
}

// parseGameID reads the game identifier from the path, answering the client itself
// when it is not usable.
func parseGameID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeInvalidID, "id must be a positive integer")
		return 0, false
	}
	return id, true
}

// findParticipant returns the participation of the given deck, or nil.
func findParticipant(game *model.Game, deckID int64) *model.GameDeck {
	for _, participant := range game.Decks {
		if participant.DeckID == deckID {
			return participant
		}
	}
	return nil
}

// resultTypeIDFor turns a result name into its reference, with nil meaning "no result
// recorded" - which is what an empty name asks for.
func resultTypeIDFor(r *http.Request, store *storage.Store, name string) (*int64, error) {
	if name == "" {
		return nil, nil
	}

	ids, err := store.Game().ResultTypeIDs(r.Context())
	if err != nil {
		return nil, err
	}
	id, ok := ids[name]
	if !ok {
		// The migration seeds win, loss and draw, so this is a database that was
		// changed by hand rather than a bad request.
		return nil, errors.New("result type " + name + " is missing from the database")
	}
	return &id, nil
}
