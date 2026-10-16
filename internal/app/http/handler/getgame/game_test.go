package getgame

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

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
	mux.HandleFunc("GET /games", ListGames(store))
	mux.HandleFunc("GET /games/{id}", GetGame(store))
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

func mustCreateGame(t *testing.T, store *storage.Store, game *model.Game) *model.Game {
	t.Helper()

	created, err := store.Game().Create(context.Background(), game)
	if err != nil {
		t.Fatalf("Create game: %v", err)
	}
	return created
}

func get(t *testing.T, mux *http.ServeMux, target string) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	return recorder
}

func decodeGames(t *testing.T, recorder *httptest.ResponseRecorder) []model.Game {
	t.Helper()

	var games []model.Game
	if err := json.Unmarshal(recorder.Body.Bytes(), &games); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body, err)
	}
	return games
}

// TestListGames_NewestFirstWithParticipants covers the shape of the listing: the
// games arrive newest first and already carry their participants.
func TestListGames_NewestFirstWithParticipants(t *testing.T) {
	store := newTestStore(t)
	mine := mustCreateDeck(t, store, "My Deck", "Andrew Kornilov")
	opponent := mustCreateDeck(t, store, "Opponent", "")

	mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-02-07",
		Decks:    []*model.GameDeck{{DeckID: mine.ID, Player: "Andrew Kornilov"}, {DeckID: opponent.ID}},
	})
	mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-10-10", Notes: "combo",
		Decks: []*model.GameDeck{{DeckID: mine.ID, Player: "Andrew Kornilov"}, {DeckID: opponent.ID}},
	})

	mux := newTestMux(store)
	recorder := get(t, mux, "/games")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusOK, recorder.Body)
	}

	games := decodeGames(t, recorder)
	if len(games) != 2 {
		t.Fatalf("games = %d, want 2", len(games))
	}
	if games[0].PlayedAt != "2026-10-10" || games[1].PlayedAt != "2026-02-07" {
		t.Errorf("order = %q then %q, want newest first", games[0].PlayedAt, games[1].PlayedAt)
	}
	for i, game := range games {
		if len(game.Decks) != 2 {
			t.Errorf("game %d has %d participants, want 2", i, len(game.Decks))
		}
		// The listing carries the names, so a client can render it without a second
		// request for the deck list.
		if game.Decks[0].DeckName != "My Deck" || game.Decks[1].DeckName != "Opponent" {
			t.Errorf("game %d participants = %+v, want their deck names", i, game.Decks)
		}
	}
}

func TestListGames_Filters(t *testing.T) {
	store := newTestStore(t)
	mine := mustCreateDeck(t, store, "My Deck", "Andrew Kornilov")
	opponent := mustCreateDeck(t, store, "Opponent", "")
	other := mustCreateDeck(t, store, "Other Opponent", "")

	mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-02-07",
		Decks:    []*model.GameDeck{{DeckID: mine.ID, Player: "Andrew Kornilov"}, {DeckID: opponent.ID}},
	})
	mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-05-23",
		Decks: []*model.GameDeck{
			{DeckID: mine.ID, Player: "Andrew Kornilov"},
			{DeckID: other.ID},
			{DeckID: opponent.ID},
		},
	})

	mux := newTestMux(store)

	for name, tc := range map[string]struct {
		target string
		want   int
	}{
		"by deck":           {fmt.Sprintf("/games?deck_id=%d", opponent.ID), 2},
		"by deck of my own": {fmt.Sprintf("/games?deck_id=%d", mine.ID), 2},
		"by player":         {"/games?player=Andrew+Kornilov", 2},
		"by unknown player": {"/games?player=Nobody", 0},
		"from a date":       {"/games?from=2026-03-01", 1},
		"to a date":         {"/games?to=2026-03-01", 1},
		"inside a range":    {"/games?from=2026-01-01&to=2026-12-31", 2},
		"outside a range":   {"/games?from=2027-01-01", 0},
		"pagination":        {"/games?limit=1", 1},
		"second page":       {"/games?limit=1&page=2", 1},
		"page past the end": {"/games?page=5", 0},
	} {
		t.Run(name, func(t *testing.T) {
			recorder := get(t, mux, tc.target)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusOK, recorder.Body)
			}
			if got := len(decodeGames(t, recorder)); got != tc.want {
				t.Errorf("games = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestListGames_RejectsBadQuery(t *testing.T) {
	store := newTestStore(t)
	mux := newTestMux(store)

	for name, target := range map[string]string{
		"page is not a number":    "/games?page=abc",
		"page is zero":            "/games?page=0",
		"limit is zero":           "/games?limit=0",
		"limit above the max":     "/games?limit=101",
		"deck id is not a number": "/games?deck_id=abc",
		"deck id is zero":         "/games?deck_id=0",
		"from is not a date":      "/games?from=01.02.26",
		"to is impossible":        "/games?to=2026-02-30",
	} {
		t.Run(name, func(t *testing.T) {
			recorder := get(t, mux, target)
			if recorder.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d (body: %s)", recorder.Code, http.StatusBadRequest, recorder.Body)
			}
		})
	}
}

func TestGetGame(t *testing.T) {
	store := newTestStore(t)
	mine := mustCreateDeck(t, store, "My Deck", "Andrew Kornilov")
	opponent := mustCreateDeck(t, store, "Opponent", "")
	game := mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-02-13", Notes: "combat damage",
		Decks: []*model.GameDeck{
			{DeckID: mine.ID, Player: "Andrew Kornilov", ResultTypeID: resultID(t, store, "loss")},
			{DeckID: opponent.ID},
		},
	})

	mux := newTestMux(store)
	recorder := get(t, mux, fmt.Sprintf("/games/%d", game.ID))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusOK, recorder.Body)
	}

	var got model.Game
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.PlayedAt != "2026-02-13" || got.Notes != "combat damage" || len(got.Decks) != 2 {
		t.Errorf("game = %+v", got)
	}
	if got.Decks[0].ResultTypeID == nil {
		t.Error("my participation lost its result")
	}
	if got.Decks[0].DeckName != "My Deck" || got.Decks[1].DeckName != "Opponent" {
		t.Errorf("participants = %+v, want their deck names", got.Decks)
	}
}

func TestGetGame_NotFound(t *testing.T) {
	store := newTestStore(t)
	mux := newTestMux(store)

	recorder := get(t, mux, "/games/999999")
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d (body: %s)", recorder.Code, http.StatusNotFound, recorder.Body)
	}
}

func TestGetGame_BadID(t *testing.T) {
	store := newTestStore(t)
	mux := newTestMux(store)

	for _, target := range []string{"/games/abc", "/games/0", "/games/-5"} {
		recorder := get(t, mux, target)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d, want %d", target, recorder.Code, http.StatusBadRequest)
		}
	}
}

func resultID(t *testing.T, store *storage.Store, name string) *int64 {
	t.Helper()

	ids, err := store.Game().ResultTypeIDs(context.Background())
	if err != nil {
		t.Fatalf("ResultTypeIDs: %v", err)
	}
	id, ok := ids[name]
	if !ok {
		t.Fatalf("result type %q is missing", name)
	}
	return &id
}
