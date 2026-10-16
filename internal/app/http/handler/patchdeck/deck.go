// Package patchdeck implements PATCH /decks/{id}.
package patchdeck

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/thelastdolphin/edh-stats/internal/app/edherrors"
	httpUtils "github.com/thelastdolphin/edh-stats/internal/app/http"
	"github.com/thelastdolphin/edh-stats/internal/app/http/dto"
	"github.com/thelastdolphin/edh-stats/internal/app/storage"
)

// maxBodyBytes caps the request body, for the same reason POST /decks caps it.
const maxBodyBytes = 1 << 20 // 1 MiB

// Deck handles PATCH /decks/{id}.
//
// Only the fields present in the body are changed; every other field keeps its
// stored value, which is what makes this a partial update and not a replacement.
func Deck(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeInvalidID, "id must be a positive integer")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

		decoder := json.NewDecoder(r.Body)
		// Refuse unknown fields: a misspelled field should fail loudly instead of
		// being silently dropped, which for a PATCH would look like success.
		decoder.DisallowUnknownFields()

		var req dto.PatchDeckRequest
		if err := decoder.Decode(&req); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				httpUtils.Error(w, http.StatusRequestEntityTooLarge, httpUtils.CodeBodyTooLarge,
					fmt.Sprintf("request body must not exceed %d bytes", maxBodyBytes))
				return
			}
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeInvalidJSON, "invalid JSON body: "+err.Error())
			return
		}

		// Anything but EOF here means a second JSON document was sent.
		if err := decoder.Decode(new(struct{})); !errors.Is(err, io.EOF) {
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeInvalidJSON, "body must contain a single JSON object")
			return
		}

		req.Normalize()
		if err := req.Validate(); err != nil {
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeValidationFailed, err.Error())
			return
		}

		// The stored deck is read first: it carries the values of the fields the
		// request does not mention, and a missing deck is answered here instead of
		// by the update.
		deck, err := store.Deck().GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, edherrors.ErrDeckNotFound) {
				httpUtils.Error(w, http.StatusNotFound, httpUtils.CodeDeckNotFound, "deck not found")
				return
			}
			log.Printf("read deck %d: %v", id, err)
			httpUtils.Error(w, http.StatusInternalServerError, httpUtils.CodeInternal, "internal error")
			return
		}

		req.Apply(deck)

		if err := store.Deck().Update(r.Context(), deck); err != nil {
			switch {
			case errors.Is(err, edherrors.ErrDeckDuplicate):
				httpUtils.Error(w, http.StatusConflict, httpUtils.CodeDeckDuplicate, "deck with the same name and owner already exists")
			case errors.Is(err, edherrors.ErrDeckNotFound):
				httpUtils.Error(w, http.StatusNotFound, httpUtils.CodeDeckNotFound, "deck not found")
			default:
				// Log the cause and answer with a generic message: SQL text is not
				// the client's business.
				log.Printf("update deck %d: %v", id, err)
				httpUtils.Error(w, http.StatusInternalServerError, httpUtils.CodeInternal, "internal error")
			}
			return
		}

		httpUtils.JSONEncode(w, http.StatusOK, deck)
	}
}
