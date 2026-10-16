package dto

import (
	"strings"
	"testing"

	"github.com/thelastdolphin/edh-stats/internal/app/model"
)

func TestPatchGameRequest(t *testing.T) {
	t.Run("an empty patch is refused", func(t *testing.T) {
		var req PatchGameRequest
		if err := req.Validate(); err == nil {
			t.Error("an empty patch was accepted")
		}
	})

	t.Run("each field may be sent on its own", func(t *testing.T) {
		for name, req := range map[string]PatchGameRequest{
			"the date":       {PlayedAt: strPtr("2026-02-07")},
			"the notes":      {Notes: strPtr("combo")},
			"a cleared note": {Notes: strPtr("")},
		} {
			if err := req.Validate(); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		}
	})

	t.Run("bad payloads are refused", func(t *testing.T) {
		for name, req := range map[string]PatchGameRequest{
			"an impossible date":       {PlayedAt: strPtr("2026-02-30")},
			"a date in the wrong form": {PlayedAt: strPtr("07.02.26")},
			"an empty date":            {PlayedAt: strPtr("")},
			"notes that are too long":  {Notes: strPtr(strings.Repeat("a", MaxNotesLen+1))},
		} {
			if err := req.Validate(); err == nil {
				t.Errorf("%s: expected an error", name)
			}
		}
	})

	t.Run("apply touches only what it carries", func(t *testing.T) {
		game := &model.Game{ID: 7, PlayedAt: "2026-02-07", Notes: "combo"}
		date := " 2026-05-23 "
		req := PatchGameRequest{PlayedAt: &date}

		req.Normalize()
		req.Apply(game)

		if game.PlayedAt != "2026-05-23" {
			t.Errorf("played_at = %q, want the trimmed new date", game.PlayedAt)
		}
		if game.Notes != "combo" || game.ID != 7 {
			t.Errorf("Apply changed something it did not carry: %+v", game)
		}
	})
}

func TestPatchGameDeckRequest(t *testing.T) {
	t.Run("an empty patch is refused", func(t *testing.T) {
		var req PatchGameDeckRequest
		if err := req.Validate(); err == nil {
			t.Error("an empty patch was accepted")
		}
	})

	t.Run("an empty string clears a value", func(t *testing.T) {
		for name, req := range map[string]PatchGameDeckRequest{
			"the player": {Player: strPtr("")},
			"the result": {Result: strPtr("")},
		} {
			if err := req.Validate(); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		}
	})

	t.Run("the result is lower-cased", func(t *testing.T) {
		req := PatchGameDeckRequest{Result: strPtr("  WIN ")}

		req.Normalize()

		if req.Result == nil || *req.Result != ResultWin {
			t.Errorf("result = %v, want %q", req.Result, ResultWin)
		}
	})

	t.Run("bad payloads are refused", func(t *testing.T) {
		for name, req := range map[string]PatchGameDeckRequest{
			"an unknown result": {Result: strPtr("victory")},
			"a player too long": {Player: strPtr(strings.Repeat("a", MaxPlayerLen+1))},
		} {
			if err := req.Validate(); err == nil {
				t.Errorf("%s: expected an error", name)
			}
		}
	})
}
