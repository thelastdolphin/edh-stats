// Package deletegame implements DELETE /games/{id}.
package deletegame

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/thelastdolphin/edh-stats/internal/app/edherrors"
	httpUtils "github.com/thelastdolphin/edh-stats/internal/app/http"
	"github.com/thelastdolphin/edh-stats/internal/app/storage"
)

// Game handles DELETE /games/{id} and answers 204 on success.
//
// The participants follow through ON DELETE CASCADE, so a wrongly recorded game is
// removed completely and nothing is left pointing at it.
func Game(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeInvalidID, "id must be a positive integer")
			return
		}

		if err := store.Game().Delete(r.Context(), id); err != nil {
			if errors.Is(err, edherrors.ErrGameNotFound) {
				httpUtils.Error(w, http.StatusNotFound, httpUtils.CodeGameNotFound, "game not found")
				return
			}
			log.Printf("delete game %d: %v", id, err)
			httpUtils.Error(w, http.StatusInternalServerError, httpUtils.CodeInternal, "internal error")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
