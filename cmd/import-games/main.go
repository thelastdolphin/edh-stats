// Command import-games loads the Markdown game log into the database.
//
// It is a one-off tool rather than part of the service: the log is a personal
// note-taking convention, and that convention has no business in the domain layer.
//
// Without -apply the tool writes nothing. It prints what it would do - including
// every deck it would create and every name it would map - so that the plan can be
// read before a single row is written.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/thelastdolphin/edh-stats/internal/app/model"
	"github.com/thelastdolphin/edh-stats/internal/app/storage"
)

type options struct {
	filePath string
	dbPath   string
	me       string
	apply    bool
	force    bool
}

func main() {
	log.SetFlags(0)

	filePath := flag.String("file", "", "the Markdown game log to read (required)")
	dbPath := flag.String("db", filepath.Join("data", "edh_stats.db"), "the SQLite database to write to")
	me := flag.String("me", "Andrew Kornilov", "owner of the decks the log is about")
	apply := flag.Bool("apply", false, "write to the database; without it the tool only reports")
	force := flag.Bool("force", false, "write even when the database already holds games")
	flag.Parse()

	if *filePath == "" {
		flag.Usage()
		log.Fatal("import-games: -file is required")
	}

	err := run(context.Background(), options{
		filePath: *filePath,
		dbPath:   *dbPath,
		me:       *me,
		apply:    *apply,
		force:    *force,
	})
	if err != nil {
		log.Fatalf("import-games: %v", err)
	}
}

func run(ctx context.Context, opts options) error {
	raw, err := os.ReadFile(opts.filePath)
	if err != nil {
		return fmt.Errorf("read the game log: %w", err)
	}

	entries, err := parseLog(string(raw))
	if err != nil {
		return fmt.Errorf("%s: %w", opts.filePath, err)
	}

	store, err := storage.NewStore(ctx, opts.dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	existing, err := store.Game().Count(ctx, storage.GameFilters{})
	if err != nil {
		return err
	}

	planned, err := buildPlan(ctx, store, entries, opts.me)
	if err != nil {
		return err
	}

	report(os.Stdout, opts, entries, planned, existing)

	if !opts.apply {
		fmt.Fprintln(os.Stdout, "\nnothing was written: pass -apply to import")
		return nil
	}
	if existing > 0 && !opts.force {
		return fmt.Errorf("the database already holds %d game(s): pass -force to import anyway", existing)
	}
	return applyPlan(ctx, store, planned)
}

// report prints the plan: what was read, which decks it will use, which it will
// create, and every game with its participants.
func report(out io.Writer, opts options, entries []entry, planned *plan, existing int) {
	fmt.Fprintf(out, "log:      %s\n", opts.filePath)
	fmt.Fprintf(out, "database: %s (%d game(s) already stored)\n", opts.dbPath, existing)
	if !opts.apply {
		fmt.Fprintf(out, "mode:     read-only\n")
	}
	fmt.Fprintf(out, "owner:    %s\n\n", opts.me)

	dates := map[string]bool{}
	opponents := 0
	for _, e := range entries {
		dates[e.date] = true
		opponents += len(e.opponents)
	}
	fmt.Fprintf(out, "%d dates, %d games, %d own participations, %d opponent participations\n\n",
		len(dates), len(entries), len(entries), opponents)

	fmt.Fprintln(out, "decks to use")
	for _, owner := range []struct {
		title string
		value string
	}{
		{"own", opts.me},
		{"opponents", ""},
	} {
		fmt.Fprintf(out, "  %s (owner %q)\n", owner.title, owner.value)
		for _, planned := range planned.decks {
			if planned.deck.Owner != owner.value {
				continue
			}
			switch {
			case planned.isNew:
				fmt.Fprintf(out, "    new      %s", planned.deck.Name)
				if planned.logName != planned.deck.Name {
					fmt.Fprintf(out, "   (the log calls it %q)", planned.logName)
				}
				fmt.Fprintln(out)
			default:
				fmt.Fprintf(out, "    id %-5d %s", planned.deck.ID, planned.deck.Name)
				if planned.logName != planned.deck.Name {
					fmt.Fprintf(out, "   (the log calls it %q)", planned.logName)
				}
				fmt.Fprintln(out)
			}
		}
	}

	fmt.Fprintln(out, "\ngames")
	for _, game := range planned.games {
		participants := make([]string, 0, len(game.participants))
		result := ""
		for i, participant := range game.participants {
			if i == 0 {
				result = participant.result
			}
			participants = append(participants, participant.deck.Name)
		}

		fmt.Fprintf(out, "  %s #%d  %-4s %s", game.entry.date, game.entry.seq, strings.ToUpper(result),
			strings.Join(participants, " vs "))
		if game.notes != "" {
			fmt.Fprintf(out, "   [%s]", game.notes)
		}
		fmt.Fprintln(out)
	}

	if len(planned.warnings) > 0 {
		fmt.Fprintln(out, "\nwarnings")
		for _, warning := range planned.warnings {
			fmt.Fprintf(out, "  %s\n", warning)
		}
	}
}

// applyPlan writes the plan: the decks that do not exist yet, then the games.
func applyPlan(ctx context.Context, store *storage.Store, planned *plan) error {
	resultIDs, err := store.Game().ResultTypeIDs(ctx)
	if err != nil {
		return err
	}

	createdDecks := 0
	for _, deck := range planned.decks {
		if !deck.isNew {
			continue
		}
		// Create fills the ID in on the pointer passed to it, which is the same one
		// the planned participants hold, so they pick it up automatically.
		if _, err := store.Deck().Create(ctx, deck.deck); err != nil {
			return fmt.Errorf("create deck %q: %w", deck.deck.Name, err)
		}
		createdDecks++
	}

	for _, game := range planned.games {
		record := &model.Game{PlayedAt: game.entry.date, Notes: game.notes}
		for _, participant := range game.participants {
			gameDeck := &model.GameDeck{DeckID: participant.deck.ID, Player: participant.player}
			if participant.result != "" {
				id, ok := resultIDs[participant.result]
				if !ok {
					return fmt.Errorf("%s #%d: unknown result %q", game.entry.date, game.entry.seq, participant.result)
				}
				gameDeck.ResultTypeID = &id
			}
			record.Decks = append(record.Decks, gameDeck)
		}

		if _, err := store.Game().Create(ctx, record); err != nil {
			return fmt.Errorf("%s #%d: %w", game.entry.date, game.entry.seq, err)
		}
	}

	log.Printf("imported %d deck(s) and %d game(s)", createdDecks, len(planned.games))
	return nil
}
