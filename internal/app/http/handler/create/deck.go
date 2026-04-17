package create

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	httpUtils "example.com/edh-stats/internal/app/http"
	"example.com/edh-stats/internal/app/http/dto"
	"example.com/edh-stats/internal/app/model"
	"example.com/edh-stats/internal/app/storage"
)

func Deck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req dto.CreateDeckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpUtils.Error(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Name == "" || req.Owner == "" {
		httpUtils.Error(w, http.StatusBadRequest, "name and owner are required")
		log.Printf("got empty name or owner")
		return
	}

	deckModel := model.Deck{
		Name:  req.Name,
		Owner: req.Owner,
		Type:  req.Type,
	}

	var deckRepo storage.DeckRepository
	deckModelPtr, err := deckRepo.Create(r.Context(), &deckModel)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("error creating deck: %v", err)
		return
	}
	httpUtils.JSONEncode(w, http.StatusCreated, deckModelPtr)
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("error closing body: %v", err)
		}
	}(r.Body)
}
