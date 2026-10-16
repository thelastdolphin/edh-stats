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

func TestGameRepository_List(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	mine := mustCreateDeck(t, store, "My Deck", "tester", "control")
	opponent := mustCreateDeck(t, store, "Opponent", "", "")

	first := mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-02-07",
		Decks:    []*model.GameDeck{{DeckID: mine.ID, Player: "tester"}, {DeckID: opponent.ID}},
	})
	second := mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-10-10", Notes: "combo",
		Decks: []*model.GameDeck{{DeckID: mine.ID, Player: "tester"}, {DeckID: opponent.ID}},
	})

	games, err := store.Game().List(ctx, GameFilters{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(games) != 2 {
		t.Fatalf("games = %d, want 2", len(games))
	}
	if games[0].ID != second.ID || games[1].ID != first.ID {
		t.Errorf("order = %d then %d, want newest first", games[0].ID, games[1].ID)
	}
	if len(games[0].Decks) != 2 {
		t.Errorf("participants of the first game = %d, want 2", len(games[0].Decks))
	}
	if games[0].Notes != "combo" {
		t.Errorf("notes = %q, want combo", games[0].Notes)
	}

	// An empty result has to be an empty slice, so that it encodes as [] and not null.
	none, err := store.Game().List(ctx, GameFilters{Player: "nobody"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if none == nil {
		t.Error("an empty listing returned nil instead of an empty slice")
	}
	if len(none) != 0 {
		t.Errorf("games for an unknown player = %d, want 0", len(none))
	}
}

func TestGameRepository_List_Filters(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	mine := mustCreateDeck(t, store, "My Deck", "tester", "control")
	opponent := mustCreateDeck(t, store, "Opponent", "", "")
	other := mustCreateDeck(t, store, "Other", "", "")

	mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-02-07",
		Decks:    []*model.GameDeck{{DeckID: mine.ID, Player: "tester"}, {DeckID: opponent.ID}},
	})
	mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-05-23",
		Decks: []*model.GameDeck{
			{DeckID: mine.ID, Player: "tester"},
			{DeckID: other.ID},
			{DeckID: opponent.ID},
		},
	})

	for name, tc := range map[string]struct {
		filters GameFilters
		want    int
	}{
		"by deck":           {GameFilters{DeckID: opponent.ID}, 2},
		"by other deck":     {GameFilters{DeckID: other.ID}, 1},
		"by player":         {GameFilters{Player: "tester"}, 2},
		"by unknown":        {GameFilters{DeckID: 999999}, 0},
		"from a date":       {GameFilters{From: "2026-03-01"}, 1},
		"to a date":         {GameFilters{To: "2026-03-01"}, 1},
		"a range":           {GameFilters{From: "2026-01-01", To: "2026-12-31"}, 2},
		"outside a range":   {GameFilters{From: "2027-01-01"}, 0},
		"first page":        {GameFilters{Limit: 1}, 1},
		"second page":       {GameFilters{Limit: 1, Page: 2}, 1},
		"page past the end": {GameFilters{Page: 5}, 0},
	} {
		t.Run(name, func(t *testing.T) {
			games, err := store.Game().List(ctx, tc.filters)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(games) != tc.want {
				t.Errorf("games = %d, want %d", len(games), tc.want)
			}
		})
	}
}

// TestGameRepository_Create_UnknownDeck is the error the API maps to 400: the only
// foreign key a participation can break is deck_id, and it must not surface as a
// driver error.
func TestGameRepository_Create_UnknownDeck(t *testing.T) {
	store := newTestStore(t)
	deck := mustCreateDeck(t, store, "My Deck", "tester", "control")

	_, err := store.Game().Create(context.Background(), &model.Game{
		PlayedAt: "2026-02-07",
		Decks:    []*model.GameDeck{{DeckID: deck.ID}, {DeckID: 999999}},
	})
	if !errors.Is(err, edherrors.ErrDeckNotFound) {
		t.Errorf("error = %v, want it to wrap edherrors.ErrDeckNotFound", err)
	}
}

func resultTypeID(t *testing.T, store *Store, name string) *int64 {
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

func TestGameRepository_DeckStats(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	mine := mustCreateDeck(t, store, "My Deck", "tester", "control")
	opponent := mustCreateDeck(t, store, "Opponent", "", "")
	mustCreateDeck(t, store, "Never Played", "", "")

	win := resultTypeID(t, store, "win")
	loss := resultTypeID(t, store, "loss")

	for _, game := range []*model.Game{
		{PlayedAt: "2026-02-07", Decks: []*model.GameDeck{
			{DeckID: mine.ID, Player: "tester", ResultTypeID: win},
			{DeckID: opponent.ID},
		}},
		{PlayedAt: "2026-05-23", Decks: []*model.GameDeck{
			{DeckID: mine.ID, Player: "tester", ResultTypeID: loss},
			{DeckID: opponent.ID},
		}},
		{PlayedAt: "2026-10-10", Decks: []*model.GameDeck{
			{DeckID: mine.ID, Player: "tester"},
			{DeckID: opponent.ID},
		}},
	} {
		mustCreateGame(t, store, game)
	}

	stats, err := store.Game().DeckStats(ctx, StatsFilters{})
	if err != nil {
		t.Fatalf("DeckStats: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("rows = %d, want 2: a deck that never played has no statistics", len(stats))
	}
	if got := stats[0]; got.DeckID != mine.ID || got.Games != 3 || got.Wins != 1 || got.Losses != 1 || got.Draws != 0 {
		t.Errorf("my statistics = %+v, want 3 games, 1 win, 1 loss", got)
	}
	if got := stats[1]; got.DeckID != opponent.ID || got.Games != 3 || got.Wins != 0 || got.Losses != 0 {
		t.Errorf("opponent statistics = %+v, want 3 games and nothing recorded", got)
	}
	// A deck without an owner is reported with an empty one, not as a failure.
	if stats[1].Owner != "" {
		t.Errorf("owner = %q, want empty", stats[1].Owner)
	}
}

func TestGameRepository_DeckStats_DateRange(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	mine := mustCreateDeck(t, store, "My Deck", "tester", "control")
	opponent := mustCreateDeck(t, store, "Opponent", "", "")

	for _, date := range []string{"2026-02-07", "2026-05-23", "2026-10-10"} {
		mustCreateGame(t, store, &model.Game{
			PlayedAt: date,
			Decks:    []*model.GameDeck{{DeckID: mine.ID}, {DeckID: opponent.ID}},
		})
	}

	for name, tc := range map[string]struct {
		filters StatsFilters
		want    int
	}{
		"everything":       {StatsFilters{}, 3},
		"from a date":      {StatsFilters{From: "2026-05-23"}, 2},
		"to a date":        {StatsFilters{To: "2026-02-07"}, 1},
		"a closed range":   {StatsFilters{From: "2026-01-01", To: "2026-12-31"}, 3},
		"outside the data": {StatsFilters{From: "2027-01-01"}, 0},
	} {
		t.Run(name, func(t *testing.T) {
			stats, err := store.Game().DeckStats(ctx, tc.filters)
			if err != nil {
				t.Fatalf("DeckStats: %v", err)
			}
			if tc.want == 0 {
				if len(stats) != 0 {
					t.Errorf("rows = %d, want none", len(stats))
				}
				return
			}
			if len(stats) == 0 || stats[0].Games != tc.want {
				t.Errorf("statistics = %+v, want %d games", stats, tc.want)
			}
		})
	}
}

func TestGameRepository_Update(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	deck := mustCreateDeck(t, store, "My Deck", "tester", "control")
	game := mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-02-07", Notes: "combo",
		Decks: []*model.GameDeck{{DeckID: deck.ID, Player: "tester"}},
	})

	game.PlayedAt = "2026-05-23"
	game.Notes = "combat damage"
	if err := store.Game().Update(ctx, game); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := store.Game().GetByID(ctx, game.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.PlayedAt != "2026-05-23" || got.Notes != "combat damage" {
		t.Errorf("game = %+v, want the new date and notes", got)
	}
	if len(got.Decks) != 1 {
		t.Errorf("participants = %d, want the update to leave them alone", len(got.Decks))
	}
}

func TestGameRepository_Update_NotFound(t *testing.T) {
	store := newTestStore(t)

	err := store.Game().Update(context.Background(), &model.Game{ID: 999999, PlayedAt: "2026-02-07"})
	if !errors.Is(err, edherrors.ErrGameNotFound) {
		t.Errorf("error = %v, want it to wrap edherrors.ErrGameNotFound", err)
	}
}

func TestGameRepository_UpdateParticipant(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	mine := mustCreateDeck(t, store, "My Deck", "tester", "control")
	opponent := mustCreateDeck(t, store, "Opponent", "", "")
	game := mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-02-07",
		Decks:    []*model.GameDeck{{DeckID: mine.ID, Player: "tester"}, {DeckID: opponent.ID}},
	})

	loss := resultTypeID(t, store, "loss")
	if err := store.Game().UpdateParticipant(ctx, game.ID, opponent.ID, "Ivan", loss); err != nil {
		t.Fatalf("UpdateParticipant: %v", err)
	}

	got, err := store.Game().GetByID(ctx, game.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Decks[1].Player != "Ivan" || got.Decks[1].ResultTypeID == nil || *got.Decks[1].ResultTypeID != *loss {
		t.Errorf("participation = %+v", got.Decks[1])
	}
	if got.Decks[0].Player != "tester" {
		t.Error("the other participation was changed")
	}

	// nil clears the result: a value here, not "leave it alone".
	if err := store.Game().UpdateParticipant(ctx, game.ID, opponent.ID, "Ivan", nil); err != nil {
		t.Fatalf("UpdateParticipant: %v", err)
	}
	got, err = store.Game().GetByID(ctx, game.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Decks[1].ResultTypeID != nil {
		t.Errorf("the result was not cleared: %+v", got.Decks[1])
	}
}

func TestGameRepository_UpdateParticipant_NotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	first := mustCreateDeck(t, store, "First", "tester", "control")
	second := mustCreateDeck(t, store, "Second", "", "")
	game := mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-02-07",
		Decks:    []*model.GameDeck{{DeckID: first.ID}, {DeckID: second.ID}},
	})

	err := store.Game().UpdateParticipant(ctx, game.ID, 999999, "Ivan", nil)
	if !errors.Is(err, edherrors.ErrParticipantNotFound) {
		t.Errorf("error = %v, want it to wrap edherrors.ErrParticipantNotFound", err)
	}

	// A deck of another game is not a participation of this one either.
	other := mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-02-08",
		Decks:    []*model.GameDeck{{DeckID: first.ID}, {DeckID: second.ID}},
	})
	stranger := mustCreateDeck(t, store, "Stranger", "", "")
	err = store.Game().UpdateParticipant(ctx, other.ID, stranger.ID, "Ivan", nil)
	if !errors.Is(err, edherrors.ErrParticipantNotFound) {
		t.Errorf("error = %v, want it to wrap edherrors.ErrParticipantNotFound", err)
	}
}

// TestGameRepository_FillsDeckNames covers the join behind the participants: a client
// listing games gets the names without asking for the deck list as well.
func TestGameRepository_FillsDeckNames(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	mine := mustCreateDeck(t, store, "My Deck", "tester", "control")
	opponent := mustCreateDeck(t, store, "Opponent", "", "")
	game := mustCreateGame(t, store, &model.Game{
		PlayedAt: "2026-02-07",
		Decks:    []*model.GameDeck{{DeckID: mine.ID}, {DeckID: opponent.ID}},
	})

	got, err := store.Game().GetByID(ctx, game.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Decks[0].DeckName != "My Deck" || got.Decks[0].DeckOwner != "tester" {
		t.Errorf("first participant = %+v", got.Decks[0])
	}
	// A deck without an owner reports an empty one rather than failing to scan.
	if got.Decks[1].DeckName != "Opponent" || got.Decks[1].DeckOwner != "" {
		t.Errorf("second participant = %+v", got.Decks[1])
	}
}
