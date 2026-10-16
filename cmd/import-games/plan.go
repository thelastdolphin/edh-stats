package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/thelastdolphin/edh-stats/internal/app/model"
	"github.com/thelastdolphin/edh-stats/internal/app/storage"
)

// aliases maps the deck names used in the log to the canonical names.
//
// This is a table, and deliberately not a similarity heuristic: the log holds
// names that differ by a single letter ("Xirys Wheels" for "Xyris Wheels") next to
// names that look just as close but are different decks. "Mothman Precon" and
// "Mothman Mill" share a commander and belong to two different players, so a
// heuristic would merge exactly the pair that has to stay apart.
var aliases = map[string]string{
	"Xirys Wheels":    "Xyris Wheels",
	"Blim Donate":     "Blim Gifts",
	"Prosper":         "Prosper Exile",
	"Eladamri Stompy": "Eladamri Goodstuff",
	"Sen Triplets":    "Sen Triplets Theft",
	// The row in the database was created under an older name, while the log uses
	// what the deck is called now. Renaming the row is a separate decision, so the
	// import keeps pointing at the deck that already exists.
	"Clement Bounce": "Clement Frogs",
}

// plan is everything the tool intends to write.
type plan struct {
	// decks are the decks the games need: the ones that already exist, followed by
	// the ones that have to be created.
	decks    []*plannedDeck
	games    []*plannedGame
	warnings []string
}

// plannedDeck is a deck the games need: either one that exists, or one to create.
type plannedDeck struct {
	logName string // the name as written in the log
	deck    *model.Deck
	isNew   bool
}

type plannedGame struct {
	entry        entry
	notes        string
	participants []*plannedParticipant
}

type plannedParticipant struct {
	deck   *model.Deck
	player string
	result string
}

// planner resolves the names of the log to decks, and remembers what it resolved.
type planner struct {
	me       string
	byName   map[deckKey]*plannedDeck
	anyOwner map[string][]string // deck name -> the owners it is registered for
	decks    []*plannedDeck
	games    []*plannedGame
	warned   map[string]bool
	warnings []string
}

type deckKey struct {
	name  string
	owner string
}

// buildPlan turns the entries of the log into the decks and games to write.
func buildPlan(ctx context.Context, store *storage.Store, entries []entry, me string) (*plan, error) {
	existing, err := loadDecks(ctx, store)
	if err != nil {
		return nil, err
	}

	p := &planner{
		me:       me,
		byName:   make(map[deckKey]*plannedDeck, len(existing)),
		anyOwner: make(map[string][]string),
		warned:   make(map[string]bool),
	}

	// A deck is identified by name and owner, which is also what the schema's
	// UNIQUE constraint says. Two rows sharing both would make every later decision
	// ambiguous, so that is reported instead of guessed at.
	for _, deck := range existing {
		key := deckKey{name: deck.Name, owner: deck.Owner}
		if _, clash := p.byName[key]; clash {
			return nil, fmt.Errorf("the database holds more than one deck called %q owned by %q", deck.Name, deck.Owner)
		}
		// Existing decks belong in the plan as well: the report has to show which
		// row a name of the log was resolved to, not only what will be created.
		planned := &plannedDeck{logName: deck.Name, deck: deck}
		p.byName[key] = planned
		p.decks = append(p.decks, planned)
		p.anyOwner[deck.Name] = append(p.anyOwner[deck.Name], deck.Owner)
	}

	for _, game := range entries {
		planned, err := p.planGame(game)
		if err != nil {
			return nil, err
		}
		p.games = append(p.games, planned)
	}

	return &plan{decks: p.decks, games: p.games, warnings: p.warnings}, nil
}

func (p *planner) planGame(game entry) (*plannedGame, error) {
	planned := &plannedGame{entry: game, notes: game.notes}

	used := make(map[*model.Deck]string)
	mine, err := p.resolve(game.myDeck, p.me, true)
	if err != nil {
		return nil, err
	}
	used[mine.deck] = game.myDeck
	planned.participants = append(planned.participants, &plannedParticipant{
		deck:   mine.deck,
		player: p.me,
		result: game.result,
	})

	for _, name := range game.opponents {
		opponent, err := p.resolve(name, "", false)
		if err != nil {
			return nil, err
		}
		if previous, clash := used[opponent.deck]; clash {
			return nil, fmt.Errorf("%s #%d: %q and %q are the same deck (%s)",
				game.date, game.seq, previous, name, opponent.deck.Name)
		}
		used[opponent.deck] = name
		planned.participants = append(planned.participants, &plannedParticipant{deck: opponent.deck})
	}

	if game.missingOpponent {
		planned.notes = strings.TrimSpace(planned.notes + "; fourth player not recorded")
		p.warn(fmt.Sprintf("%s #%d ends with a bare %q: importing %d participants and noting it in the game notes",
			game.date, game.seq, "vs", len(planned.participants)))
	}
	return planned, nil
}

// resolve returns the deck for a name in the log, creating a planned deck when the
// database has none yet.
func (p *planner) resolve(logName, owner string, mine bool) (*plannedDeck, error) {
	name := logName
	if canonical, ok := aliases[logName]; ok {
		name = canonical
	}

	key := deckKey{name: name, owner: owner}
	if planned, ok := p.byName[key]; ok {
		// Remember how the log spells it, so that the report can show which row a
		// name of the log was resolved to, not only what will be created.
		if planned.logName == planned.deck.Name && logName != planned.deck.Name {
			planned.logName = logName
		}
		return planned, nil
	}

	// Creating a deck of mine while a deck with the same name is registered for
	// somebody else almost always means -me does not match the database.
	if mine {
		for _, other := range p.anyOwner[name] {
			p.warn(fmt.Sprintf("the log's deck %q is not registered for owner %q, but a deck of that name exists for %q: check -me",
				logName, owner, other))
		}
	}

	planned := &plannedDeck{
		logName: logName,
		deck:    &model.Deck{Name: name, Owner: owner},
		isNew:   true,
	}
	p.byName[key] = planned
	p.decks = append(p.decks, planned)
	p.anyOwner[name] = append(p.anyOwner[name], owner)
	return planned, nil
}

func (p *planner) warn(message string) {
	if p.warned[message] {
		return
	}
	p.warned[message] = true
	p.warnings = append(p.warnings, message)
}

// loadDecks reads every deck, paging through the listing.
func loadDecks(ctx context.Context, store *storage.Store) ([]*model.Deck, error) {
	var all []*model.Deck
	for page := 1; ; page++ {
		batch, err := store.Deck().List(ctx, storage.DeckFilters{Page: page, Limit: storage.MaxLimit})
		if err != nil {
			return nil, fmt.Errorf("list decks: %w", err)
		}
		all = append(all, batch...)
		if len(batch) < storage.MaxLimit {
			return all, nil
		}
	}
}
