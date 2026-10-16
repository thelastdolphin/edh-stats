package dto

import (
	"strings"
	"testing"

	"github.com/thelastdolphin/edh-stats/internal/app/model"
)

func strPtr(value string) *string {
	return &value
}

func TestPatchDeckRequest_EmptyPayloadIsRefused(t *testing.T) {
	var req PatchDeckRequest

	if err := req.Validate(); err == nil {
		t.Error("an empty patch was accepted; it is almost always a broken client")
	}
}

func TestPatchDeckRequest_ValidatesOnlyWhatItCarries(t *testing.T) {
	for name, tc := range map[string]struct {
		request PatchDeckRequest
		wantErr bool
	}{
		"name only":        {PatchDeckRequest{Name: strPtr("Deck")}, false},
		"owner only":       {PatchDeckRequest{Owner: strPtr("Andrew")}, false},
		"type only":        {PatchDeckRequest{Type: strPtr("combo")}, false},
		"all three":        {PatchDeckRequest{Name: strPtr("Deck"), Owner: strPtr("Andrew"), Type: strPtr("combo")}, false},
		"50 runes is fine": {PatchDeckRequest{Name: strPtr(strings.Repeat("я", MaxNameLen))}, false},
		"51 runes is not":  {PatchDeckRequest{Name: strPtr(strings.Repeat("я", MaxNameLen+1))}, true},
		"empty name":       {PatchDeckRequest{Name: strPtr("")}, true},
		"empty owner":      {PatchDeckRequest{Owner: strPtr("")}, true},
		"empty type":       {PatchDeckRequest{Type: strPtr("")}, true},
		"owner too long":   {PatchDeckRequest{Owner: strPtr(strings.Repeat("a", MaxOwnerLen+1))}, true},
		"type too long":    {PatchDeckRequest{Type: strPtr(strings.Repeat("a", MaxTypeLen+1))}, true},
	} {
		t.Run(name, func(t *testing.T) {
			err := tc.request.Validate()
			if tc.wantErr && err == nil {
				t.Error("expected an error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestPatchDeckRequest_NormalizeMakesBlanksEmpty covers the order the handler uses:
// trimming first, so that "   " is refused as an empty name instead of being
// stored as whitespace.
func TestPatchDeckRequest_NormalizeMakesBlanksEmpty(t *testing.T) {
	req := PatchDeckRequest{Name: strPtr("   ")}

	req.Normalize()

	if req.Name == nil {
		t.Fatal("Normalize dropped the field instead of trimming it")
	}
	if *req.Name != "" {
		t.Errorf("name = %q, want it trimmed to nothing", *req.Name)
	}
	if err := req.Validate(); err == nil {
		t.Error("a blank name passed validation")
	}
}

func TestPatchDeckRequest_NormalizeTrimsWhatItKeeps(t *testing.T) {
	req := PatchDeckRequest{Name: strPtr("  Deck  "), Owner: strPtr(" Andrew ")}

	req.Normalize()

	if *req.Name != "Deck" || *req.Owner != "Andrew" {
		t.Errorf("normalized = %q / %q", *req.Name, *req.Owner)
	}
	if err := req.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestPatchDeckRequest_ApplyTouchesOnlyWhatItCarries(t *testing.T) {
	deck := &model.Deck{ID: 1, Name: "Deck", Owner: "Andrew", Type: "control"}

	PatchDeckRequest{Type: strPtr("combo")}.Apply(deck)

	if deck.Name != "Deck" || deck.Owner != "Andrew" {
		t.Errorf("Apply changed a field the request did not carry: %+v", deck)
	}
	if deck.Type != "combo" {
		t.Errorf("type = %q, want combo", deck.Type)
	}
	if deck.ID != 1 {
		t.Errorf("id = %d, want it untouched", deck.ID)
	}
}
