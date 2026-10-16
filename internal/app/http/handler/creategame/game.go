// Package creategame implements POST /games.
package creategame

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

// maxBodyBytes caps the request body, for the same reason POST /decks caps it.
const maxBodyBytes = 1 << 20 // 1 MiB

// Game handles POST /games.
//
// The method itself is enforced by the "POST /games" routing pattern, so no manual
// method check is needed here.
func Game(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

		decoder := json.NewDecoder(r.Body)
		// Refuse unknown fields: a misspelled field should fail loudly instead of
		// being silently dropped.
		decoder.DisallowUnknownFields()

		var req dto.CreateGameRequest
		if err := decoder.Decode(&req); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				httpUtils.Error(w, http.StatusRequestEntityTooLarge,
					fmt.Sprintf("request body must not exceed %d bytes", maxBodyBytes))
				return
			}
			httpUtils.Error(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
			return
		}

		// Anything but EOF here means a second JSON document was sent.
		if err := decoder.Decode(new(struct{})); !errors.Is(err, io.EOF) {
			httpUtils.Error(w, http.StatusBadRequest, "body must contain a single JSON object")
			return
		}

		req.Normalize()
		if err := req.Validate(); err != nil {
			httpUtils.Error(w, http.StatusBadRequest, err.Error())
			return
		}

		// The result is stored as a reference, so the names from the request are
		// looked up once for the whole payload.
		resultIDs, err := store.Game().ResultTypeIDs(r.Context())
		if err != nil {
			log.Printf("create game: %v", err)
			httpUtils.Error(w, http.StatusInternalServerError, "internal error")
			return
		}

		game := &model.Game{PlayedAt: req.PlayedAt, Notes: req.Notes}
		for _, participant := range req.Decks {
			entry := &model.GameDeck{DeckID: participant.DeckID, Player: participant.Player}
			if participant.Result != "" {
				id, ok := resultIDs[participant.Result]
				if !ok {
					// The migration seeds them, so a missing name means a database
					// that was tampered with rather than a bad request.
					log.Printf("create game: result type %q is missing from the database", participant.Result)
					httpUtils.Error(w, http.StatusInternalServerError, "internal error")
					return
				}
				entry.ResultTypeID = &id
			}
			game.Decks = append(game.Decks, entry)
		}

		created, err := store.Game().Create(r.Context(), game)
		if err != nil {
			switch {
			case errors.Is(err, edherrors.ErrDeckNotFound):
				httpUtils.Error(w, http.StatusBadRequest, "a deck in the request does not exist")
			case errors.Is(err, edherrors.ErrDeckAlreadyInGame):
				httpUtils.Error(w, http.StatusBadRequest, "the same deck appears more than once")
			default:
				log.Printf("create game: %v", err)
				httpUtils.Error(w, http.StatusInternalServerError, "internal error")
			}
			return
		}

		w.Header().Set("Location", "/games/"+strconv.FormatInt(created.ID, 10))
		httpUtils.JSONEncode(w, http.StatusCreated, created)
	}
}
