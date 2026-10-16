package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/thelastdolphin/edh-stats/internal/app/model"
)

// entry is one game as written in the log.
type entry struct {
	line      int
	date      string // ISO 8601, "2026-02-07"
	seq       int
	myDeck    string
	result    string // win, loss or draw
	opponents []string
	notes     string
	// missingOpponent marks a line that ends with a bare "vs": somebody took part
	// but their deck was not written down.
	missingOpponent bool
}

var (
	dateLine    = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{2}$`)
	gameLine    = regexp.MustCompile(`^(\d+)\)\s*(.+)$`)
	notesSuffix = regexp.MustCompile(`\s*\(([^()]*)\)$`)
)

// parseLog reads the Markdown game log.
//
// A date stands on its own line and the games below it are numbered:
//
//	07.02.26
//	1) Marchesa Aikido Win vs Inspirit Artifacts vs Eladamri Stompy (combat damage)
//
// The result is the last word before the first "vs", not the first word of the
// line that happens to look like one. The log contains a deck called "Arjun Draw",
// where the obvious reading takes "Draw" for the result and "Loss" for an opponent.
//
// The trailing parenthesis is the note of the game; it is optional.
func parseLog(text string) ([]entry, error) {
	var (
		entries []entry
		date    string
	)

	for number, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		if dateLine.MatchString(line) {
			parsed, err := time.Parse("02.01.06", line)
			if err != nil {
				return nil, fmt.Errorf("line %d: %q is not a date", number+1, line)
			}
			date = parsed.Format(model.DateLayout)
			continue
		}

		match := gameLine.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("line %d: cannot read %q", number+1, line)
		}
		if date == "" {
			return nil, fmt.Errorf("line %d: %q comes before any date", number+1, line)
		}

		seq, err := strconv.Atoi(match[1])
		if err != nil {
			return nil, fmt.Errorf("line %d: %q is not a game number", number+1, match[1])
		}

		game, err := parseGame(match[2])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", number+1, err)
		}
		game.line = number + 1
		game.date = date
		game.seq = seq
		entries = append(entries, game)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no games found")
	}
	return entries, nil
}

// parseGame reads the body of a game line, that is everything after "1)".
func parseGame(body string) (entry, error) {
	var game entry

	if match := notesSuffix.FindStringSubmatch(body); match != nil {
		game.notes = strings.TrimSpace(match[1])
		body = strings.TrimSpace(strings.TrimSuffix(body, match[0]))
	}

	head, tail, found := strings.Cut(body, " vs ")
	if !found {
		return game, fmt.Errorf("%q has no opponents", body)
	}

	// The result is the last word before the first "vs".
	cut := strings.LastIndexAny(head, " \t")
	if cut < 0 {
		return game, fmt.Errorf("%q has no result", head)
	}
	game.myDeck = strings.TrimSpace(head[:cut])
	game.result = strings.ToLower(strings.TrimSpace(head[cut+1:]))
	if game.myDeck == "" {
		return game, fmt.Errorf("%q has no deck name", head)
	}
	switch game.result {
	case "win", "loss", "draw":
	default:
		return game, fmt.Errorf("%q does not end with win, loss or draw", head)
	}

	// A trailing "vs" is a player whose deck was not written down.
	if trimmed := strings.TrimSuffix(tail, " vs"); trimmed != tail {
		tail = trimmed
		game.missingOpponent = true
	}

	for _, opponent := range strings.Split(tail, " vs ") {
		opponent = strings.TrimSpace(opponent)
		if opponent == "" {
			return game, fmt.Errorf("%q has an opponent without a deck name", body)
		}
		game.opponents = append(game.opponents, opponent)
	}
	if len(game.opponents) == 0 {
		return game, fmt.Errorf("%q has no opponents", body)
	}
	return game, nil
}
