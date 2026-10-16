package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thelastdolphin/edh-stats/internal/app/model"
	"github.com/thelastdolphin/edh-stats/internal/app/storage"
)

const testOwner = "Andrew Kornilov"

func newTestStore(t *testing.T) *storage.Store {
	t.Helper()

	store, err := storage.NewStore(context.Background(), filepath.Join(t.TempDir(), "test.db"))
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

func mustCreateDeck(t *testing.T, store *storage.Store, name, owner string) *model.Deck {
	t.Helper()

	deck, err := store.Deck().Create(context.Background(), &model.Deck{Name: name, Owner: owner})
	if err != nil {
		t.Fatalf("Create deck %q: %v", name, err)
	}
	return deck
}

// deckByName returns the planned deck whose canonical name is name.
func deckByName(t *testing.T, planned *plan, name string) *plannedDeck {
	t.Helper()

	for _, deck := range planned.decks {
		if deck.deck.Name == name {
			return deck
		}
	}
	t.Fatalf("the plan has no deck called %q (it has %v)", name, plannedNames(planned))
	return nil
}

func plannedNames(planned *plan) []string {
	names := make([]string, 0, len(planned.decks))
	for _, deck := range planned.decks {
		names = append(names, deck.deck.Name)
	}
	return names
}

func TestBuildPlan_ResolvesExistingDecksAndPlansNewOnes(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// The database holds the decks under their canonical names, while the log uses
	// the older spellings.
	frogs := mustCreateDeck(t, store, "Clement Frogs", testOwner)
	xyris := mustCreateDeck(t, store, "Xyris Wheels", "")

	entries := []entry{
		{
			date: "2026-02-07", seq: 1, myDeck: "Clement Bounce", result: "win",
			opponents: []string{"Sixth Doctor Historic Copies", "Xirys Wheels"},
			notes:     "opps conceded",
		},
		{
			date: "2026-02-13", seq: 1, myDeck: "Liesa Lifegain/Lifeloss", result: "loss",
			opponents: []string{"Xirys Wheels", "Miirym Dragons"},
			notes:     "combat damage", missingOpponent: true,
		},
	}

	planned, err := buildPlan(ctx, store, entries, testOwner)
	if err != nil {
		t.Fatalf("buildPlan: %v", err)
	}

	t.Run("existing decks are reused", func(t *testing.T) {
		resolved := deckByName(t, planned, "Clement Frogs")
		if resolved.isNew {
			t.Error("Clement Frogs should have been found in the database")
		}
		if resolved.deck.ID != frogs.ID {
			t.Errorf("Clement Frogs id = %d, want %d", resolved.deck.ID, frogs.ID)
		}
		if resolved.logName != "Clement Bounce" {
			t.Errorf("logName = %q, want %q", resolved.logName, "Clement Bounce")
		}

		opponent := deckByName(t, planned, "Xyris Wheels")
		if opponent.isNew || opponent.deck.ID != xyris.ID {
			t.Errorf("Xyris Wheels was not resolved to the existing deck: %+v", opponent)
		}
		if opponent.logName != "Xirys Wheels" {
			t.Errorf("logName = %q, want %q", opponent.logName, "Xirys Wheels")
		}
	})

	t.Run("unknown decks are planned with the right owner", func(t *testing.T) {
		for name, wantOwner := range map[string]string{
			"Sixth Doctor Historic Copies": "",
			"Miirym Dragons":               "",
			"Liesa Lifegain/Lifeloss":      testOwner,
		} {
			deck := deckByName(t, planned, name)
			if !deck.isNew {
				t.Errorf("%s should be created", name)
			}
			if deck.deck.Owner != wantOwner {
				t.Errorf("%s owner = %q, want %q", name, deck.deck.Owner, wantOwner)
			}
		}
	})

	t.Run("participants carry the result and the player only for my deck", func(t *testing.T) {
		game := planned.games[0]
		if len(game.participants) != 3 {
			t.Fatalf("participants = %d, want 3", len(game.participants))
		}
		if game.participants[0].player != testOwner || game.participants[0].result != "win" {
			t.Errorf("my participation = %+v", game.participants[0])
		}
		for _, i := range []int{1, 2} {
			if game.participants[i].player != "" || game.participants[i].result != "" {
				t.Errorf("opponent participation %d = %+v, want no player and no result", i, game.participants[i])
			}
		}
	})

	t.Run("a missing opponent is noted and warned about", func(t *testing.T) {
		game := planned.games[1]
		if !strings.Contains(game.notes, "fourth player not recorded") {
			t.Errorf("notes = %q, want the missing player to be recorded", game.notes)
		}
		if !strings.HasPrefix(game.notes, "combat damage") {
			t.Errorf("notes = %q, want the original note to be kept", game.notes)
		}

		var warned bool
		for _, warning := range planned.warnings {
			if strings.Contains(warning, "bare") {
				warned = true
			}
		}
		if !warned {
			t.Errorf("warnings = %v, want one about the bare vs", planned.warnings)
		}
	})
}

// TestBuildPlan_WarnsAboutAMismatchedOwner guards the -me flag: creating decks for
// the wrong owner would duplicate every deck of the log.
func TestBuildPlan_WarnsAboutAMismatchedOwner(t *testing.T) {
	store := newTestStore(t)
	mustCreateDeck(t, store, "Marchesa Aikido", testOwner)

	planned, err := buildPlan(context.Background(), store,
		[]entry{{date: "2026-03-14", seq: 1, myDeck: "Marchesa Aikido", result: "win", opponents: []string{"Xyris Wheels"}}},
		"andrew")
	if err != nil {
		t.Fatalf("buildPlan: %v", err)
	}

	var warned bool
	for _, warning := range planned.warnings {
		if strings.Contains(warning, "check -me") {
			warned = true
		}
	}
	if !warned {
		t.Errorf("warnings = %v, want one about the owner", planned.warnings)
	}
}

// TestBuildPlan_RejectsTheSameDeckTwiceInOneGame covers two names of the log that
// resolve to one deck: the pair cannot be stored, because a participation is
// identified by the game and the deck.
func TestBuildPlan_RejectsTheSameDeckTwiceInOneGame(t *testing.T) {
	store := newTestStore(t)

	_, err := buildPlan(context.Background(), store, []entry{{
		date: "2026-05-23", seq: 2, myDeck: "Kenrith Vote", result: "win",
		opponents: []string{"Blim Donate", "Blim Gifts"},
	}}, testOwner)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "are the same deck") {
		t.Errorf("error = %q, want it to explain the clash", err)
	}
}

func TestApplyPlan_WritesDecksAndGames(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	entries := []entry{
		{date: "2026-02-07", seq: 1, myDeck: "Clement Bounce", result: "win",
			opponents: []string{"Sixth Doctor Historic Copies", "Magar Spells"}, notes: "opps conceded"},
		{date: "2026-02-07", seq: 2, myDeck: "Clement Bounce", result: "loss",
			opponents: []string{"Miereke Steal"}},
	}

	planned, err := buildPlan(ctx, store, entries, testOwner)
	if err != nil {
		t.Fatalf("buildPlan: %v", err)
	}
	if err := applyPlan(ctx, store, planned); err != nil {
		t.Fatalf("applyPlan: %v", err)
	}

	count, err := store.Game().Count(ctx, storage.GameFilters{})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Fatalf("games stored = %d, want 2", count)
	}

	// The deck created by the import is then reusable by name.
	found, err := store.Deck().List(ctx, storage.DeckFilters{Name: "Clement Frogs", Owner: testOwner})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("Clement Frogs = %d row(s), want 1", len(found))
	}

	game, err := store.Game().GetByID(ctx, 1)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if game.PlayedAt != "2026-02-07" || game.Notes != "opps conceded" {
		t.Errorf("game = %+v", game)
	}
	if len(game.Decks) != 3 {
		t.Fatalf("participants = %d, want 3", len(game.Decks))
	}
	if game.Decks[0].DeckID != found[0].ID {
		t.Errorf("my deck = %d, want %d", game.Decks[0].DeckID, found[0].ID)
	}
	if game.Decks[0].Player != testOwner {
		t.Errorf("player = %q, want %q", game.Decks[0].Player, testOwner)
	}
	if game.Decks[0].ResultTypeID == nil {
		t.Error("my participation has no result")
	}
	if game.Decks[1].ResultTypeID != nil {
		t.Error("an opponent should have no result")
	}
}
