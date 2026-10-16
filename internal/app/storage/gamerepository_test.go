package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/thelastdolphin/edh-stats/internal/app/edherrors"
	"github.com/thelastdolphin/edh-stats/internal/app/model"
)

func int64Ptr(value int64) *int64 {
	return &value
}

// mustCreateGame stores the game and fails the test if the storage layer refuses it.
func mustCreateGame(t *testing.T, store *Store, game *model.Game) *model.Game {
	t.Helper()

	created, err := store.Game().Create(context.Background(), game)
	if err != nil {
		t.Fatalf("Create game: %v", err)
	}
	return created
}

func TestGameRepository_CreateAndGetByID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	mine := mustCreateDeck(t, store, "My Deck", "Andrew", "control")
	first := mustCreateDeck(t, store, "First Opponent", "", "")
	second := mustCreateDeck(t, store, "Second Opponent", "", "")

	created := mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-02-07",
		Notes:    "conceded; combat damage",
		Decks: []*model.GameDeck{
			{DeckID: mine.ID, Player: "Andrew", ResultTypeID: int64Ptr(1)},
			{DeckID: first.ID},
			{DeckID: second.ID},
		},
	})
	if created.ID == 0 {
		t.Fatal("expected the generated game ID to be set")
	}

	got, err := store.Game().GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if got.PlayedAt != "2026-02-07" {
		t.Errorf("PlayedAt = %q, want %q", got.PlayedAt, "2026-02-07")
	}
	if got.Notes != "conceded; combat damage" {
		t.Errorf("Notes = %q", got.Notes)
	}
	if len(got.Decks) != 3 {
		t.Fatalf("participants = %d, want 3", len(got.Decks))
	}

	// Order matters: it is the order the game was recorded in.
	for i, wantDeckID := range []int64{mine.ID, first.ID, second.ID} {
		if got.Decks[i].DeckID != wantDeckID {
			t.Errorf("participant %d = deck %d, want %d", i, got.Decks[i].DeckID, wantDeckID)
		}
	}

	if got.Decks[0].Player != "Andrew" {
		t.Errorf("player of my deck = %q, want %q", got.Decks[0].Player, "Andrew")
	}
	if got.Decks[0].ResultTypeID == nil || *got.Decks[0].ResultTypeID != 1 {
		t.Errorf("result of my deck = %v, want 1", got.Decks[0].ResultTypeID)
	}

	for _, i := range []int{1, 2} {
		if got.Decks[i].Player != "" {
			t.Errorf("participant %d player = %q, want empty", i, got.Decks[i].Player)
		}
		if got.Decks[i].ResultTypeID != nil {
			t.Errorf("participant %d result = %v, want none", i, *got.Decks[i].ResultTypeID)
		}
	}
}

// TestGameRepository_Create_ResultAndNotesAreOptional is the shape the imported
// games have: a result only for the owner's deck, and no notes at all.
func TestGameRepository_Create_ResultAndNotesAreOptional(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	mine := mustCreateDeck(t, store, "My Deck", "Andrew", "control")
	opponent := mustCreateDeck(t, store, "Opponent", "", "")

	created := mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-08-16",
		Decks: []*model.GameDeck{
			{DeckID: mine.ID, Player: "Andrew", ResultTypeID: int64Ptr(2)},
			{DeckID: opponent.ID},
		},
	})

	got, err := store.Game().GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Notes != "" {
		t.Errorf("Notes = %q, want empty", got.Notes)
	}
	if got.Decks[1].ResultTypeID != nil {
		t.Error("the opponent should have no result")
	}
}

// TestGameRepository_Create_IsAtomic covers the transaction: a game that cannot be
// stored completely must not be stored at all.
func TestGameRepository_Create_IsAtomic(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	deck := mustCreateDeck(t, store, "My Deck", "Andrew", "control")

	for name, game := range map[string]*model.Game{
		"the same deck twice": {
			PlayedAt: "2026-02-07",
			Decks: []*model.GameDeck{
				{DeckID: deck.ID},
				{DeckID: deck.ID},
			},
		},
		"a deck that does not exist": {
			PlayedAt: "2026-02-07",
			Decks: []*model.GameDeck{
				{DeckID: deck.ID},
				{DeckID: 999999},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := store.Game().Create(ctx, game)
			if err == nil {
				t.Fatal("expected an error")
			}

			count, err := store.Game().Count(ctx)
			if err != nil {
				t.Fatalf("Count: %v", err)
			}
			if count != 0 {
				t.Errorf("games stored = %d, want 0: the transaction left a partial game behind", count)
			}

			var participations int
			if err := store.db.QueryRowContext(ctx, `SELECT count(*) FROM game_deck`).Scan(&participations); err != nil {
				t.Fatalf("count participations: %v", err)
			}
			if participations != 0 {
				t.Errorf("participations stored = %d, want 0", participations)
			}
		})
	}

	// The duplicate has to be recognisable as such, not just as any failure.
	_, err := store.Game().Create(ctx, &model.Game{
		PlayedAt: "2026-02-07",
		Decks: []*model.GameDeck{
			{DeckID: deck.ID},
			{DeckID: deck.ID},
		},
	})
	if !errors.Is(err, edherrors.ErrDeckAlreadyInGame) {
		t.Errorf("error = %v, want it to wrap edherrors.ErrDeckAlreadyInGame", err)
	}
}

func TestGameRepository_GetByID_NotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.Game().GetByID(context.Background(), 4242)
	if !errors.Is(err, edherrors.ErrGameNotFound) {
		t.Errorf("error = %v, want it to wrap edherrors.ErrGameNotFound", err)
	}
}

// TestGameRepository_GetByID_NormalisesLegacyTimestamp covers a row written by an
// earlier release, where played_at defaulted to CURRENT_TIMESTAMP.
func TestGameRepository_GetByID_NormalisesLegacyTimestamp(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if _, err := store.db.ExecContext(ctx,
		`INSERT INTO games (played_at, notes) VALUES ('2026-10-16 02:42:14', NULL)`); err != nil {
		t.Fatalf("insert legacy game: %v", err)
	}

	got, err := store.Game().GetByID(ctx, 1)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.PlayedAt != "2026-10-16" {
		t.Errorf("PlayedAt = %q, want %q", got.PlayedAt, "2026-10-16")
	}
	if len(got.Decks) != 0 {
		t.Errorf("participants = %d, want 0", len(got.Decks))
	}
}

func TestGameRepository_Delete(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	deck := mustCreateDeck(t, store, "My Deck", "Andrew", "control")
	game := mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-02-07",
		Decks:    []*model.GameDeck{{DeckID: deck.ID, Player: "Andrew"}},
	})

	if err := store.Game().Delete(ctx, game.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if err := store.Game().Delete(ctx, game.ID); !errors.Is(err, edherrors.ErrGameNotFound) {
		t.Errorf("second delete = %v, want it to wrap edherrors.ErrGameNotFound", err)
	}

	// The participants are removed by the database, not by the repository.
	var participations int
	if err := store.db.QueryRowContext(ctx, `SELECT count(*) FROM game_deck`).Scan(&participations); err != nil {
		t.Fatalf("count participations: %v", err)
	}
	if participations != 0 {
		t.Errorf("participations left after deleting the game = %d, want 0", participations)
	}
}

func TestGameRepository_ResultTypeIDs(t *testing.T) {
	store := newTestStore(t)

	ids, err := store.Game().ResultTypeIDs(context.Background())
	if err != nil {
		t.Fatalf("ResultTypeIDs: %v", err)
	}

	for _, name := range []string{"win", "loss", "draw"} {
		if ids[name] == 0 {
			t.Errorf("result type %q is missing (got %v)", name, ids)
		}
	}
}
