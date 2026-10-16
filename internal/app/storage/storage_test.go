package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thelastdolphin/edh-stats/internal/app/edherrors"
	"github.com/thelastdolphin/edh-stats/internal/app/model"
)

// newTestStore returns a Store backed by a freshly created database.
//
// The tests deliberately go through NewStore so that the real migrations run:
// the schema under test is then the shipped one, instead of a hand-written copy
// in the test file that can silently drift away from migrations/.
func newTestStore(t *testing.T) *Store {
	t.Helper()

	store, err := NewStore(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return store
}

func mustCreateDeck(t *testing.T, store *Store, name, owner, deckType string) *model.Deck {
	t.Helper()

	deck, err := store.Deck().Create(context.Background(), &model.Deck{
		Name:  name,
		Owner: owner,
		Type:  deckType,
	})
	if err != nil {
		t.Fatalf("Create(%q, %q): %v", name, owner, err)
	}
	return deck
}

func TestNewStore_AppliesMigrations(t *testing.T) {
	store := newTestStore(t)

	const query = `SELECT count(*) FROM sqlite_master
	               WHERE type = 'table'
	                 AND name IN ('decks', 'games', 'game_deck', 'result_types')`

	var tables int
	if err := store.db.QueryRowContext(context.Background(), query).Scan(&tables); err != nil {
		t.Fatalf("count tables: %v", err)
	}
	if tables != 4 {
		t.Errorf("migrated tables = %d, want 4", tables)
	}
}

// TestNewStore_EnforcesForeignKeys is the regression test for the PRAGMA bug:
// the setting used to be applied to a single pooled connection, so foreign keys
// were enforced only when the pool happened to hand out that connection.
func TestNewStore_EnforcesForeignKeys(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	var enabled int
	if err := store.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&enabled); err != nil {
		t.Fatalf("read pragma: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("PRAGMA foreign_keys = %d, want 1", enabled)
	}

	// game_deck.game_id references games(id) and no game has been inserted.
	if _, err := store.db.ExecContext(ctx,
		`INSERT INTO game_deck (game_id, deck_id, result_type_id) VALUES (1, 1, 1)`); err == nil {
		t.Error("insert with dangling foreign keys succeeded: foreign keys are not enforced")
	}
}

// TestNewStore_AppliesPragmas covers the connection settings that were missing
// altogether: without them SQLite ran with the rollback journal and the default
// lock timeout.
func TestNewStore_AppliesPragmas(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	var journalMode string
	if err := store.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want wal", journalMode)
	}

	var busyTimeout int
	if err := store.db.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if busyTimeout != busyTimeoutMs {
		t.Errorf("busy_timeout = %d, want %d", busyTimeout, busyTimeoutMs)
	}
}

func TestDeckRepository_Create(t *testing.T) {
	store := newTestStore(t)

	deck := mustCreateDeck(t, store, "My Deck", "tester", "control")
	if deck.ID == 0 {
		t.Error("expected the generated ID to be set, got 0")
	}

	got, err := store.Deck().GetByID(context.Background(), deck.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "My Deck" || got.Owner != "tester" || got.Type != "control" {
		t.Errorf("round trip mismatch: %+v", got)
	}
}

// TestDeckRepository_Create_Duplicate covers the bug that turned a duplicate
// deck into a 500 response: the repository must report the domain error, which
// is what the handler maps to 409.
func TestDeckRepository_Create_Duplicate(t *testing.T) {
	store := newTestStore(t)
	mustCreateDeck(t, store, "Test Deck", "tester", "control")

	_, err := store.Deck().Create(context.Background(), &model.Deck{
		Name:  "Test Deck",
		Owner: "tester",
		Type:  "aggro",
	})
	if err == nil {
		t.Fatal("expected an error for a duplicate name and owner, got nil")
	}
	if !errors.Is(err, edherrors.ErrDeckDuplicate) {
		t.Errorf("error = %v, want it to wrap edherrors.ErrDeckDuplicate", err)
	}
}

func TestDeckRepository_GetByID_NotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.Deck().GetByID(context.Background(), 999999)
	if !errors.Is(err, edherrors.ErrDeckNotFound) {
		t.Errorf("error = %v, want it to wrap edherrors.ErrDeckNotFound", err)
	}
}

func TestDeckRepository_List_Pagination(t *testing.T) {
	store := newTestStore(t)
	for i := 1; i <= 5; i++ {
		mustCreateDeck(t, store, fmt.Sprintf("deck-%d", i), "tester", "control")
	}
	ctx := context.Background()

	page, err := store.Deck().List(ctx, DeckFilters{Page: 2, Limit: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("len(page) = %d, want 2", len(page))
	}
	// ORDER BY id makes the page deterministic.
	if page[0].Name != "deck-3" || page[1].Name != "deck-4" {
		t.Errorf("page 2 = %q, %q; want deck-3, deck-4", page[0].Name, page[1].Name)
	}

	lastPage, err := store.Deck().List(ctx, DeckFilters{Page: 3, Limit: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(lastPage) != 1 || lastPage[0].Name != "deck-5" {
		t.Errorf("page 3 = %+v, want only deck-5", lastPage)
	}
}

// TestDeckRepository_List_EmptyIsJSONArray guards the API contract: an empty
// result must encode as [] and not as null.
func TestDeckRepository_List_EmptyIsJSONArray(t *testing.T) {
	store := newTestStore(t)

	decks, err := store.Deck().List(context.Background(), DeckFilters{Owner: "nobody"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if decks == nil {
		t.Error("List returned a nil slice, want an empty one")
	}

	encoded, err := json.Marshal(decks)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(encoded) != "[]" {
		t.Errorf("empty list encodes as %s, want []", encoded)
	}
}

func TestDeckRepository_List_Filters(t *testing.T) {
	store := newTestStore(t)
	mustCreateDeck(t, store, "deck-a", "alice", "control")
	mustCreateDeck(t, store, "deck-b", "bob", "aggro")

	decks, err := store.Deck().List(context.Background(), DeckFilters{Owner: "bob"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(decks) != 1 || decks[0].Name != "deck-b" {
		t.Errorf("owner filter returned %+v, want only deck-b", decks)
	}
}

func TestDeckRepository_Delete(t *testing.T) {
	store := newTestStore(t)
	deck := mustCreateDeck(t, store, "deck to delete", "tester", "control")
	ctx := context.Background()

	if err := store.Deck().Delete(ctx, deck.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Deck().GetByID(ctx, deck.ID); !errors.Is(err, edherrors.ErrDeckNotFound) {
		t.Errorf("deck is still readable after the delete: %v", err)
	}
}

func TestDeckRepository_Delete_NotFound(t *testing.T) {
	store := newTestStore(t)

	err := store.Deck().Delete(context.Background(), 999999)
	if !errors.Is(err, edherrors.ErrDeckNotFound) {
		t.Errorf("error = %v, want it to wrap edherrors.ErrDeckNotFound", err)
	}
}

// TestDeckRepository_Delete_InUse covers the ON DELETE RESTRICT the schema
// declares: a deck a game still refers to must survive, and the caller sees the
// domain error rather than a driver message.
func TestDeckRepository_Delete_InUse(t *testing.T) {
	store := newTestStore(t)
	deck := mustCreateDeck(t, store, "referenced deck", "tester", "control")
	ctx := context.Background()

	result, err := store.db.ExecContext(ctx, `INSERT INTO games (notes) VALUES ('test game')`)
	if err != nil {
		t.Fatalf("insert game: %v", err)
	}
	gameID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("game id: %v", err)
	}
	if _, err := store.db.ExecContext(ctx,
		`INSERT INTO game_deck (game_id, deck_id, result_type_id) VALUES (?, ?, 1)`,
		gameID, deck.ID); err != nil {
		t.Fatalf("insert game_deck: %v", err)
	}

	if err := store.Deck().Delete(ctx, deck.ID); !errors.Is(err, edherrors.ErrDeckInUse) {
		t.Errorf("error = %v, want it to wrap edherrors.ErrDeckInUse", err)
	}
	if _, err := store.Deck().GetByID(ctx, deck.ID); err != nil {
		t.Errorf("the deck disappeared despite the restriction: %v", err)
	}
}

func TestDeckRepository_Update(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	deck := mustCreateDeck(t, store, "Old Name", "tester", "control")
	deck.Name = "New Name"
	deck.Owner = "someone else"
	deck.Type = "combo"

	if err := store.Deck().Update(ctx, deck); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := store.Deck().GetByID(ctx, deck.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "New Name" || got.Owner != "someone else" || got.Type != "combo" {
		t.Errorf("deck after the update = %+v", got)
	}
}

// TestDeckRepository_Update_UnchangedValues covers the update that changes nothing:
// the row keeps its values and the call must not look like a missing deck.
func TestDeckRepository_Update_UnchangedValues(t *testing.T) {
	store := newTestStore(t)
	deck := mustCreateDeck(t, store, "Same Name", "tester", "control")

	if err := store.Deck().Update(context.Background(), deck); err != nil {
		t.Errorf("updating a deck with its own values: %v", err)
	}
}

func TestDeckRepository_Update_Duplicate(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	first := mustCreateDeck(t, store, "First", "tester", "control")
	mustCreateDeck(t, store, "Second", "tester", "control")

	first.Name = "Second"
	err := store.Deck().Update(ctx, first)
	if !errors.Is(err, edherrors.ErrDeckDuplicate) {
		t.Errorf("error = %v, want it to wrap edherrors.ErrDeckDuplicate", err)
	}
}

func TestDeckRepository_Update_NotFound(t *testing.T) {
	store := newTestStore(t)

	err := store.Deck().Update(context.Background(),
		&model.Deck{ID: 999999, Name: "Name", Owner: "tester", Type: "control"})
	if !errors.Is(err, edherrors.ErrDeckNotFound) {
		t.Errorf("error = %v, want it to wrap edherrors.ErrDeckNotFound", err)
	}
}

func TestDeckFilters_Clamp(t *testing.T) {
	tests := []struct {
		name       string
		in         DeckFilters
		wantPage   int
		wantLimit  int
		wantOffset int
	}{
		{name: "zero values", in: DeckFilters{}, wantPage: DefaultPage, wantLimit: DefaultLimit, wantOffset: 0},
		{name: "negative", in: DeckFilters{Page: -3, Limit: -1}, wantPage: DefaultPage, wantLimit: DefaultLimit, wantOffset: 0},
		{name: "limit above max", in: DeckFilters{Page: 1, Limit: MaxLimit + 1}, wantPage: 1, wantLimit: MaxLimit, wantOffset: 0},
		{name: "second page", in: DeckFilters{Page: 2, Limit: 20}, wantPage: 2, wantLimit: 20, wantOffset: 20},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.in.Clamp()
			if got.Page != test.wantPage || got.Limit != test.wantLimit {
				t.Errorf("Clamp() = page %d limit %d, want page %d limit %d",
					got.Page, got.Limit, test.wantPage, test.wantLimit)
			}
			if offset := got.Offset(); offset != test.wantOffset {
				t.Errorf("Offset() = %d, want %d", offset, test.wantOffset)
			}
		})
	}
}

func TestDataSourceName_EnablesPragmas(t *testing.T) {
	dsn := dataSourceName("./data/edh_stats.db")

	for _, want := range []string{"_foreign_keys=on", "_journal_mode=WAL", "_busy_timeout=", "_txlock=immediate"} {
		if !strings.Contains(dsn, want) {
			t.Errorf("DSN %q is missing %q", dsn, want)
		}
	}
}
