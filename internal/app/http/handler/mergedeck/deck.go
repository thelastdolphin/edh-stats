// Package mergedeck exposes merging one deck into another.
package mergedeck

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/thelastdolphin/edh-stats/internal/app/edherrors"
	httpUtils "github.com/thelastdolphin/edh-stats/internal/app/http"
	"github.com/thelastdolphin/edh-stats/internal/app/http/dto"
	"github.com/thelastdolphin/edh-stats/internal/app/storage"
)

// Deck merges the deck named in the path into the deck named in the body.
//
// The body decides which deck survives: POST /decks/19/merge {"into": 47} moves every game
// from 19 to 47 and deletes 19. The opposite direction is not an error, it is simply a
// different answer to which of the two should stay.
func Deck(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeInvalidID,
				"id must be a positive integer")
			return
		}

		var req dto.MergeDeckRequest
		if !httpUtils.DecodeJSON(w, r, &req) {
			return
		}
		if err := req.Validate(); err != nil {
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeValidationFailed, err.Error())
			return
		}

		moved, err := store.Deck().Merge(r.Context(), id, *req.Into)
		if err != nil {
			switch {
			case errors.Is(err, edherrors.ErrDeckMergeIntoItself):
				httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeValidationFailed,
					"a deck cannot be merged into itself")
			case errors.Is(err, edherrors.ErrDeckNotFound):
				httpUtils.Error(w, http.StatusNotFound, httpUtils.CodeDeckNotFound,
					"one of the decks does not exist")
			case errors.Is(err, edherrors.ErrDeckAlreadyInGame):
				// Nothing was changed: a game that had both decks would end up with the same
				// deck twice, and dropping a participant to avoid that is not this endpoint's
				// decision to make.
				httpUtils.Error(w, http.StatusConflict, httpUtils.CodeDuplicateParticipant,
					"a game already has both decks; nothing was changed")
			default:
				log.Printf("merge deck %d into %d: %v", id, *req.Into, err)
				httpUtils.Error(w, http.StatusInternalServerError, httpUtils.CodeInternal,
					"internal error")
			}
			return
		}

		httpUtils.JSONEncode(w, http.StatusOK, dto.MergeDeckResponse{Moved: moved})
	}
}
