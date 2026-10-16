package deletegame

import (
	"context"
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
	mux.HandleFunc("DELETE /games/{id}", Game(store))
	return mux
}

func mustCreateGame(t *testing.T, store *storage.Store) *model.Game {
	t.Helper()

	ctx := context.Background()
	first, err := store.Deck().Create(ctx, &model.Deck{Name: "My Deck", Owner: "Andrew Kornilov"})
	if err != nil {
		t.Fatalf("Create deck: %v", err)
	}
	second, err := store.Deck().Create(ctx, &model.Deck{Name: "Opponent", Owner: ""})
	if err != nil {
		t.Fatalf("Create deck: %v", err)
	}

	game, err := store.Game().Create(ctx, &model.Game{
		PlayedAt: "2026-02-07",
		Decks: []*model.GameDeck{
			{DeckID: first.ID, Player: "Andrew Kornilov"},
			{DeckID: second.ID},
		},
	})
	if err != nil {
		t.Fatalf("Create game: %v", err)
	}
	return game
}

func do(t *testing.T, mux *http.ServeMux, target string) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, target, nil))
	return recorder
}

func TestGame_DeletesTheGameAndItsParticipants(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	game := mustCreateGame(t, store)
	mux := newTestMux(store)

	recorder := do(t, mux, fmt.Sprintf("/games/%d", game.ID))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusNoContent, recorder.Body)
	}
	if body := recorder.Body.String(); body != "" {
		t.Errorf("body = %q, want an empty one", body)
	}

	if _, err := store.Game().GetByID(ctx, game.ID); err == nil {
		t.Error("the game is still readable after a successful delete")
	}

	// A second delete is a 404.
	if again := do(t, mux, fmt.Sprintf("/games/%d", game.ID)); again.Code != http.StatusNotFound {
		t.Errorf("second delete = %d, want %d", again.Code, http.StatusNotFound)
	}
}

func TestGame_NotFound(t *testing.T) {
	store := newTestStore(t)
	mux := newTestMux(store)

	recorder := do(t, mux, "/games/999999")
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d (body: %s)", recorder.Code, http.StatusNotFound, recorder.Body)
	}
}

func TestGame_BadID(t *testing.T) {
	store := newTestStore(t)
	mux := newTestMux(store)

	for _, target := range []string{"/games/abc", "/games/0", "/games/-5"} {
		recorder := do(t, mux, target)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("DELETE %s = %d, want %d", target, recorder.Code, http.StatusBadRequest)
		}
	}
}
