package deletedeck

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/thelastdolphin/edh-stats/internal/app/model"
	"github.com/thelastdolphin/edh-stats/internal/app/storage"
)

// newTestStore returns a store together with the database path: the path is what
// lets a test add a game that refers to a deck, since there is no repository for
// games yet.
func newTestStore(t *testing.T) (*storage.Store, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := storage.NewStore(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("storage.NewStore: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return store, dbPath
}

// newTestMux wires the same pattern the application registers, so that
// r.PathValue("id") is populated exactly as it is in production.
func newTestMux(store *storage.Store) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /decks/{id}", Deck(store))
	return mux
}

func mustCreateDeck(t *testing.T, store *storage.Store, name string) *model.Deck {
	t.Helper()

	deck, err := store.Deck().Create(context.Background(), &model.Deck{
		Name:  name,
		Owner: "tester",
		Type:  "control",
	})
	if err != nil {
		t.Fatalf("Create(%q): %v", name, err)
	}
	return deck
}

// linkGame makes the deck undeletable by inserting a game that refers to it.
func linkGame(t *testing.T, dbPath string, deckID int64) {
	t.Helper()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open %s: %v", dbPath, err)
	}
	defer db.Close()

	result, err := db.Exec(`INSERT INTO games (notes) VALUES ('test game')`)
	if err != nil {
		t.Fatalf("insert game: %v", err)
	}
	gameID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("game id: %v", err)
	}

	// result_type_id 1 is "win", which the migration seeds.
	if _, err := db.Exec(`INSERT INTO game_deck (game_id, deck_id, result_type_id) VALUES (?, ?, 1)`,
		gameID, deckID); err != nil {
		t.Fatalf("insert game_deck: %v", err)
	}
}

func do(t *testing.T, mux *http.ServeMux, method, target string) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(method, target, nil))
	return recorder
}

func TestDeck_Deleted(t *testing.T) {
	store, _ := newTestStore(t)
	deck := mustCreateDeck(t, store, "deck to delete")
	mux := newTestMux(store)

	recorder := do(t, mux, http.MethodDelete, fmt.Sprintf("/decks/%d", deck.ID))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusNoContent, recorder.Body)
	}
	if body := recorder.Body.String(); body != "" {
		t.Errorf("body = %q, want an empty one", body)
	}

	// The deck is really gone, and a second delete is a 404.
	if _, err := store.Deck().GetByID(context.Background(), deck.ID); err == nil {
		t.Error("deck is still readable after a successful delete")
	}
	again := do(t, mux, http.MethodDelete, fmt.Sprintf("/decks/%d", deck.ID))
	if again.Code != http.StatusNotFound {
		t.Errorf("second delete = %d, want %d", again.Code, http.StatusNotFound)
	}
}

func TestDeck_NotFound(t *testing.T) {
	store, _ := newTestStore(t)
	mux := newTestMux(store)

	recorder := do(t, mux, http.MethodDelete, "/decks/999999")
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d (body: %s)", recorder.Code, http.StatusNotFound, recorder.Body)
	}
}

func TestDeck_BadID(t *testing.T) {
	store, _ := newTestStore(t)
	mux := newTestMux(store)

	for _, target := range []string{"/decks/abc", "/decks/0", "/decks/-5", "/decks/1.5"} {
		recorder := do(t, mux, http.MethodDelete, target)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("DELETE %s = %d, want %d", target, recorder.Code, http.StatusBadRequest)
		}
	}
}

// TestDeck_InUseIsConflict covers the ON DELETE RESTRICT the schema declares.
func TestDeck_InUseIsConflict(t *testing.T) {
	store, dbPath := newTestStore(t)
	deck := mustCreateDeck(t, store, "referenced deck")
	linkGame(t, dbPath, deck.ID)
	mux := newTestMux(store)

	recorder := do(t, mux, http.MethodDelete, fmt.Sprintf("/decks/%d", deck.ID))
	if recorder.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d (body: %s)", recorder.Code, http.StatusConflict, recorder.Body)
	}
	if _, err := store.Deck().GetByID(context.Background(), deck.ID); err != nil {
		t.Errorf("deck disappeared despite the restriction: %v", err)
	}
}
