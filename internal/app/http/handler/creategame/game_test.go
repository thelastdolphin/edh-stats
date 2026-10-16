package creategame

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thelastdolphin/edh-stats/internal/app/model"
	"github.com/thelastdolphin/edh-stats/internal/app/storage"
)

func newTestStore(t *testing.T) *storage.Store {
	t.Helper()

	store, err := storage.NewStore(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("storage.NewStore: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return store
}

func newTestMux(store *storage.Store) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /games", Game(store))
	return mux
}

func mustCreateDeck(t *testing.T, store *storage.Store, name, owner string) *model.Deck {
	t.Helper()

	deck, err := store.Deck().Create(context.Background(), &model.Deck{Name: name, Owner: owner})
	if err != nil {
		t.Fatalf("Create deck %q: %v", name, err)
	}
	return deck
}

func post(t *testing.T, mux *http.ServeMux, body string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/games", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}

func decodeGame(t *testing.T, recorder *httptest.ResponseRecorder) model.Game {
	t.Helper()

	var game model.Game
	if err := json.Unmarshal(recorder.Body.Bytes(), &game); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body, err)
	}
	return game
}

func TestGame_CreatesTheGame(t *testing.T) {
	store := newTestStore(t)
	mine := mustCreateDeck(t, store, "Clement Bounce", "Andrew Kornilov")
	opponent := mustCreateDeck(t, store, "Magar Spells", "")
	other := mustCreateDeck(t, store, "Wilhelt Zombies", "")
	mux := newTestMux(store)

	recorder := post(t, mux, fmt.Sprintf(`{
		"played_at": "2026-02-07",
		"notes": "opps conceded",
		"decks": [
			{"deck_id": %d, "player": "Andrew Kornilov", "result": "win"},
			{"deck_id": %d},
			{"deck_id": %d}
		]}`, mine.ID, opponent.ID, other.ID))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusCreated, recorder.Body)
	}
	if got := recorder.Header().Get("Location"); got != "/games/1" {
		t.Errorf("Location = %q, want %q", got, "/games/1")
	}

	game := decodeGame(t, recorder)
	if game.PlayedAt != "2026-02-07" || game.Notes != "opps conceded" {
		t.Errorf("game = %+v", game)
	}
	if len(game.Decks) != 3 {
		t.Fatalf("participants = %d, want 3", len(game.Decks))
	}
	// The order of the request is the order of the game.
	for i, wantDeckID := range []int64{mine.ID, opponent.ID, other.ID} {
		if game.Decks[i].DeckID != wantDeckID {
			t.Errorf("participant %d = deck %d, want %d", i, game.Decks[i].DeckID, wantDeckID)
		}
	}
	if game.Decks[0].Player != "Andrew Kornilov" || game.Decks[0].ResultTypeID == nil {
		t.Errorf("my participation = %+v", game.Decks[0])
	}
	for _, i := range []int{1, 2} {
		if game.Decks[i].Player != "" || game.Decks[i].ResultTypeID != nil {
			t.Errorf("opponent %d = %+v, want no player and no result", i, game.Decks[i])
		}
	}

	// The response is the stored game, so it carries the deck names too.
	if game.Decks[0].DeckName != "Clement Bounce" {
		t.Errorf("the response has no deck name: %+v", game.Decks[0])
	}

	stored, err := store.Game().GetByID(context.Background(), game.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(stored.Decks) != 3 || stored.Notes != "opps conceded" {
		t.Errorf("stored game = %+v", stored)
	}
}

// TestGame_WithoutADateUsesToday covers the column default that the storage layer
// applies when the request carries no date.
func TestGame_WithoutADateUsesToday(t *testing.T) {
	store := newTestStore(t)
	first := mustCreateDeck(t, store, "First", "Andrew Kornilov")
	second := mustCreateDeck(t, store, "Second", "")
	mux := newTestMux(store)

	recorder := post(t, mux, fmt.Sprintf(`{"decks":[{"deck_id":%d},{"deck_id":%d}]}`, first.ID, second.ID))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusCreated, recorder.Body)
	}

	if got, want := decodeGame(t, recorder).PlayedAt, time.Now().Format(model.DateLayout); got != want {
		t.Errorf("played_at = %q, want today (%q)", got, want)
	}
}

func TestGame_UnknownDeckIsBadRequest(t *testing.T) {
	store := newTestStore(t)
	deck := mustCreateDeck(t, store, "Deck", "Andrew Kornilov")
	mux := newTestMux(store)

	recorder := post(t, mux, fmt.Sprintf(`{"decks":[{"deck_id":%d},{"deck_id":999999}]}`, deck.ID))
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d (body: %s)", recorder.Code, http.StatusBadRequest, recorder.Body)
	}

	assertNothingStored(t, store)
}

func TestGame_RejectsBadPayloads(t *testing.T) {
	for name, body := range map[string]string{
		"no decks":            `{"decks":[]}`,
		"single deck":         `{"decks":[{"deck_id":1}]}`,
		"the same deck twice": `{"decks":[{"deck_id":1},{"deck_id":1}]}`,
		"unknown result":      `{"decks":[{"deck_id":1},{"deck_id":2,"result":"victory"}]}`,
		"impossible date":     `{"played_at":"2026-13-01","decks":[{"deck_id":1},{"deck_id":2}]}`,
		"unknown field":       `{"decks":[{"deck_id":1},{"deck_id":2}],"players":[]}`,
		"not json":            `not json`,
	} {
		t.Run(name, func(t *testing.T) {
			store := newTestStore(t)
			mustCreateDeck(t, store, "First", "Andrew Kornilov")
			mustCreateDeck(t, store, "Second", "")
			mux := newTestMux(store)

			recorder := post(t, mux, body)
			if recorder.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d (body: %s)", recorder.Code, http.StatusBadRequest, recorder.Body)
			}
			assertNothingStored(t, store)
		})
	}
}

func assertNothingStored(t *testing.T, store *storage.Store) {
	t.Helper()

	count, err := store.Game().Count(context.Background())
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Errorf("games stored = %d, want 0: a refused request must not leave anything behind", count)
	}
}
