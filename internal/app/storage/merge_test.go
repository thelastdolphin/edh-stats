package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/thelastdolphin/edh-stats/internal/app/edherrors"
	"github.com/thelastdolphin/edh-stats/internal/app/model"
)

func TestDeckRepository_MergeMovesGamesAndDeletesTheSource(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	source := mustCreateDeck(t, store, "Blim Gifts", "", "")
	target := mustCreateDeck(t, store, "Blim Gifts", "Georgy Borodin", "")

	onSource := mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-02-21",
		Decks:    []*model.GameDeck{{DeckID: source.ID, Player: "Andrew"}},
	})
	onTarget := mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-03-01",
		Decks:    []*model.GameDeck{{DeckID: target.ID, Player: "Andrew"}},
	})

	moved, err := store.Deck().Merge(ctx, source.ID, target.ID)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if moved != 1 {
		t.Fatalf("moved = %d, want 1", moved)
	}

	if _, err := store.Deck().GetByID(ctx, source.ID); !errors.Is(err, edherrors.ErrDeckNotFound) {
		t.Fatalf("the merged deck is still there: %v", err)
	}

	for _, gameID := range []int64{onSource.ID, onTarget.ID} {
		game, err := store.Game().GetByID(ctx, gameID)
		if err != nil {
			t.Fatalf("GetByID(%d): %v", gameID, err)
		}
		if len(game.Decks) != 1 || game.Decks[0].DeckID != target.ID {
			t.Fatalf("game %d does not point at the surviving deck: %+v", gameID, game.Decks)
		}
	}
}

func TestDeckRepository_MergeIntoItselfIsRefused(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	deck := mustCreateDeck(t, store, "Blim Gifts", "Georgy Borodin", "")

	if _, err := store.Deck().Merge(ctx, deck.ID, deck.ID); !errors.Is(err, edherrors.ErrDeckMergeIntoItself) {
		t.Fatalf("err = %v, want ErrDeckMergeIntoItself", err)
	}

	if _, err := store.Deck().GetByID(ctx, deck.ID); err != nil {
		t.Fatalf("the deck was harmed by a refused merge: %v", err)
	}
}

func TestDeckRepository_MergeWithAMissingDeckIsRefused(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	deck := mustCreateDeck(t, store, "Blim Gifts", "Georgy Borodin", "")

	if _, err := store.Deck().Merge(ctx, deck.ID, deck.ID+99); !errors.Is(err, edherrors.ErrDeckNotFound) {
		t.Fatalf("err = %v, want ErrDeckNotFound", err)
	}
}

func TestDeckRepository_MergeRefusesWhenGameHasBothDecks(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	source := mustCreateDeck(t, store, "Wilhelt Zombies", "", "")
	target := mustCreateDeck(t, store, "Wilhelt Zombies", "Georgy Borodin", "")

	game := mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-02-21",
		Decks: []*model.GameDeck{
			{DeckID: source.ID, Player: "Andrew"},
			{DeckID: target.ID, Player: "Georgy"},
		},
	})

	if _, err := store.Deck().Merge(ctx, source.ID, target.ID); !errors.Is(err, edherrors.ErrDeckAlreadyInGame) {
		t.Fatalf("err = %v, want ErrDeckAlreadyInGame", err)
	}

	// nothing changed: both decks exist and the game still has both participants
	if _, err := store.Deck().GetByID(ctx, source.ID); err != nil {
		t.Fatalf("the source deck disappeared after a refused merge: %v", err)
	}
	after, err := store.Game().GetByID(ctx, game.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(after.Decks) != 2 {
		t.Fatalf("the game lost participants: %+v", after.Decks)
	}
}

func TestGameRepository_MoveParticipant(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	wrong := mustCreateDeck(t, store, "Wilhelt Zombies (AA)", "Alexander Agassiz", "")
	right := mustCreateDeck(t, store, "Wilhelt Zombies", "Georgy Borodin", "")

	game := mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-08-16",
		Decks:    []*model.GameDeck{{DeckID: wrong.ID, Player: "Georgy"}},
	})

	if err := store.Game().MoveParticipant(ctx, game.ID, wrong.ID, right.ID); err != nil {
		t.Fatalf("MoveParticipant: %v", err)
	}

	after, err := store.Game().GetByID(ctx, game.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(after.Decks) != 1 || after.Decks[0].DeckID != right.ID {
		t.Fatalf("the participation did not move: %+v", after.Decks)
	}
	if after.Decks[0].Player != "Georgy" {
		t.Fatalf("the player was lost by the move: %q", after.Decks[0].Player)
	}
}

func TestGameRepository_MoveParticipantRefusals(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	played := mustCreateDeck(t, store, "Liesa Lifegain/Lifeloss", "Andrew Kornilov", "")
	other := mustCreateDeck(t, store, "Arjun Draw", "Andrew Kornilov", "")
	spare := mustCreateDeck(t, store, "Krenko Goblins", "Andrew Kornilov", "")
	missing := played.ID + 99

	game := mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-08-16",
		Decks: []*model.GameDeck{
			{DeckID: played.ID, Player: "Andrew"},
			{DeckID: other.ID, Player: "Ivan"},
		},
	})

	tests := []struct {
		name string
		from int64
		to   int64
		want error
	}{
		{"onto a deck already in the game", played.ID, other.ID, edherrors.ErrDeckAlreadyInGame},
		{"onto a deck that does not exist", played.ID, missing, edherrors.ErrDeckNotFound},
		// the target must not already be in the game, or the clash is reported instead
		{"from a deck that is not in the game", spare.ID, other.ID, edherrors.ErrDeckAlreadyInGame},
		{"from a deck that is not in the game onto a free deck", spare.ID, missing, edherrors.ErrDeckNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := store.Game().MoveParticipant(ctx, game.ID, test.from, test.to); !errors.Is(err, test.want) {
				t.Fatalf("err = %v, want %v", err, test.want)
			}
		})
	}
}
