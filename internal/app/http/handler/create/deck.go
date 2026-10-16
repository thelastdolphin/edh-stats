// Package create implements POST /decks.
package create

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
	"github.com/thelastdolphin/edh-stats/internal/app/model"
	"github.com/thelastdolphin/edh-stats/internal/app/storage"
)

// maxBodyBytes caps the request body. Without a cap a client can send an
// arbitrarily large payload (a 20 MB "name" used to be accepted and stored) and
// exhaust memory and disk.
const maxBodyBytes = 1 << 20 // 1 MiB

// Deck handles POST /decks.
//
// The method itself is enforced by the "POST /decks" routing pattern, so no
// manual method check is needed here.
func Deck(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

		decoder := json.NewDecoder(r.Body)
		// Refuse unknown fields: a misspelled field should fail loudly instead
		// of being silently dropped.
		decoder.DisallowUnknownFields()

		var req dto.CreateDeckRequest
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

		deck, err := store.Deck().Create(r.Context(), &model.Deck{
			Name:  req.Name,
			Owner: req.Owner,
			Type:  req.Type,
		})
		if err != nil {
			if errors.Is(err, edherrors.ErrDeckDuplicate) {
				httpUtils.Error(w, http.StatusConflict, httpUtils.CodeDeckDuplicate, "deck with the same name and owner already exists")
				return
			}
			// Log the cause and answer with a generic message: SQL text is not
			// the client's business.
			log.Printf("create deck: %v", err)
			httpUtils.Error(w, http.StatusInternalServerError, httpUtils.CodeInternal, "internal error")
			return
		}

		w.Header().Set("Location", "/decks/"+strconv.FormatInt(deck.ID, 10))
		httpUtils.JSONEncode(w, http.StatusCreated, deck)
	}
}
