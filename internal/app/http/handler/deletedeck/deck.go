// Package deletedeck implements DELETE /decks/{id}.
package deletedeck

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/thelastdolphin/edh-stats/internal/app/edherrors"
	httpUtils "github.com/thelastdolphin/edh-stats/internal/app/http"
	"github.com/thelastdolphin/edh-stats/internal/app/storage"
)

// Deck handles DELETE /decks/{id} and answers 204 on success.
//
// The method itself is enforced by the "DELETE /decks/{id}" routing pattern, so
// no manual method check is needed here.
func Deck(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parsed as int64 to match model.Deck.ID; zero and negative IDs can never
		// exist, so they are bad input rather than a reason to go to the database.
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			httpUtils.Error(w, http.StatusBadRequest, "id must be a positive integer")
			return
		}

		if err := store.Deck().Delete(r.Context(), id); err != nil {
			switch {
			case errors.Is(err, edherrors.ErrDeckNotFound):
				httpUtils.Error(w, http.StatusNotFound, "deck not found")
			case errors.Is(err, edherrors.ErrDeckInUse):
				// The schema refuses to drop a deck a game refers to; that is the
				// client's conflict, not a server failure.
				httpUtils.Error(w, http.StatusConflict, "deck is still referenced by a game")
			default:
				// Log the cause, answer with a generic message: SQL text is not the
				// client's business.
				log.Printf("delete deck %d: %v", id, err)
				httpUtils.Error(w, http.StatusInternalServerError, "internal error")
			}
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
