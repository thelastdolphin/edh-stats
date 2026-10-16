package main

import (
	"strings"
	"testing"
)

// logFixture is a trimmed copy of the real log and keeps its inconsistencies on
// purpose: one result is spelled in lower case, one line ends with a bare "vs",
// one game has no note, and one deck name contains the word "Draw".
const logFixture = `07.02.26
1) Clement Bounce Win vs Sixth Doctor Historic Copies vs Magar Spells (opps conceded)
2) Liesa Lifegain/Lifeloss Win vs Miereke Steal vs Magar Spells (combo)

13.02.26
1) Phenax Toughness/Mill Loss vs Xirys Wheels vs Miirym Dragons vs (combat damage)
2) Doctor Who Historic Copies Loss vs Calix Enchantress vs Prosper vs Judith (spell damage)

16.08.26
1) Kydele/Ravos Schism Loss vs Wilhelt Zombies vs Mothman Mill vs Zurzoth Devils/Discard

10.10.26
1) Arjun Draw Loss vs Zurzoth Devils/Discard vs Mm'menon Artifacts (combo; non-combat damage)
3) Liesa Lifegain/Lifeloss win vs Pantlaza Dinosaurs vs Blim Gifts vs Magar Spells (non-combat damage)
`

func TestParseLog(t *testing.T) {
	entries, err := parseLog(logFixture)
	if err != nil {
		t.Fatalf("parseLog: %v", err)
	}
	if len(entries) != 7 {
		t.Fatalf("entries = %d, want 7", len(entries))
	}

	first := entries[0]
	if first.date != "2026-02-07" || first.seq != 1 {
		t.Errorf("first entry = %s #%d, want 2026-02-07 #1", first.date, first.seq)
	}
	if first.myDeck != "Clement Bounce" {
		t.Errorf("myDeck = %q, want %q", first.myDeck, "Clement Bounce")
	}
	if first.result != "win" {
		t.Errorf("result = %q, want win", first.result)
	}
	if got := strings.Join(first.opponents, "|"); got != "Sixth Doctor Historic Copies|Magar Spells" {
		t.Errorf("opponents = %q", got)
	}
	if first.notes != "opps conceded" {
		t.Errorf("notes = %q", first.notes)
	}
	if first.missingOpponent {
		t.Error("missingOpponent = true for a complete line")
	}
	// The line number is there to point at the log when something is wrong.
	if first.line != 2 {
		t.Errorf("line = %d, want 2", first.line)
	}
}

// TestParseLog_DrawInsideADeckName is the reason the result is read from the end of
// the head: taking the first result-looking word turns "Arjun Draw" into a draw for
// a deck called "Arjun", and "Loss" into an opponent.
func TestParseLog_DrawInsideADeckName(t *testing.T) {
	entries, err := parseLog(logFixture)
	if err != nil {
		t.Fatalf("parseLog: %v", err)
	}

	var found bool
	for _, e := range entries {
		if e.date != "2026-10-10" || e.seq != 1 {
			continue
		}
		found = true
		if e.myDeck != "Arjun Draw" {
			t.Errorf("myDeck = %q, want %q", e.myDeck, "Arjun Draw")
		}
		if e.result != "loss" {
			t.Errorf("result = %q, want loss", e.result)
		}
		if got := strings.Join(e.opponents, "|"); got != "Zurzoth Devils/Discard|Mm'menon Artifacts" {
			t.Errorf("opponents = %q", got)
		}
		if e.notes != "combo; non-combat damage" {
			t.Errorf("notes = %q", e.notes)
		}
	}
	if !found {
		t.Fatal("the game with Draw in the deck name was not parsed")
	}
}

func TestParseLog_MissingOpponent(t *testing.T) {
	entries, err := parseLog(logFixture)
	if err != nil {
		t.Fatalf("parseLog: %v", err)
	}

	for _, e := range entries {
		if e.date != "2026-02-13" || e.seq != 1 {
			continue
		}
		if !e.missingOpponent {
			t.Error("missingOpponent = false, want true")
		}
		if got := strings.Join(e.opponents, "|"); got != "Xirys Wheels|Miirym Dragons" {
			t.Errorf("opponents = %q, want the bare trailing vs to be dropped", got)
		}
		return
	}
	t.Fatal("the game with the missing opponent was not parsed")
}

func TestParseLog_LowerCaseResult(t *testing.T) {
	entries, err := parseLog("10.10.26\n3) Liesa Lifegain/Lifeloss win vs Pantlaza Dinosaurs (non-combat damage)\n")
	if err != nil {
		t.Fatalf("parseLog: %v", err)
	}
	if entries[0].result != "win" {
		t.Errorf("result = %q, want win", entries[0].result)
	}
}

func TestParseLog_Errors(t *testing.T) {
	for name, tc := range map[string]struct {
		text string
		want string
	}{
		"no games":            {"", "no games found"},
		"before any date":     {"1) Clement Bounce Win vs Magar Spells", "before any date"},
		"unreadable line":     {"07.02.26\nsomething else", "cannot read"},
		"impossible date":     {"32.13.26\n1) A Win vs B", "not a date"},
		"one word, no result": {"07.02.26\n1) Clement vs Magar Spells", "has no result"},
		"no result word":      {"07.02.26\n1) Clement Bounce vs Magar Spells", "does not end with win, loss or draw"},
		"unknown result word": {"07.02.26\n1) Clement Bounce Victory vs Magar Spells", "does not end with win, loss or draw"},
		"no opponents":        {"07.02.26\n1) Clement Bounce Win", "has no opponents"},
		"empty opponent":      {"07.02.26\n1) Clement Bounce Win vs  vs Magar Spells", "opponent without a deck name"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseLog(tc.text)
			if err == nil {
				t.Fatalf("expected an error mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}
