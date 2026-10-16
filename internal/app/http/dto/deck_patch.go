package dto

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/thelastdolphin/edh-stats/internal/app/model"
)

// PatchDeckRequest is the JSON body of PATCH /decks/{id}.
//
// The fields are pointers on purpose: a PATCH changes only what the request
// carries, so "not mentioned" has to be distinguishable from "sent as an empty
// value". With plain strings a body of {} would wipe the deck.
type PatchDeckRequest struct {
	Name  *string `json:"name"`
	Owner *string `json:"owner"`
	Type  *string `json:"type"`
}

// Normalize trims the fields the request carries, for the same reason
// CreateDeckRequest does: " Deck " and "Deck" are the same deck and must not be
// able to slip past the name+owner uniqueness rule.
func (r *PatchDeckRequest) Normalize() {
	r.Name = trimmed(r.Name)
	r.Owner = trimmed(r.Owner)
	r.Type = trimmed(r.Type)
}

// Validate reports the first problem found in the payload.
//
// An empty request is refused rather than treated as a no-op: it is almost always
// a client that built the body wrong, and answering 200 would hide that.
func (r PatchDeckRequest) Validate() error {
	if r.Name == nil && r.Owner == nil && r.Type == nil {
		return errors.New("provide at least one of name, owner or type")
	}

	if r.Name != nil {
		if *r.Name == "" {
			return errors.New("name must not be empty")
		}
		if utf8.RuneCountInString(*r.Name) > MaxNameLen {
			return fmt.Errorf("name must be at most %d characters", MaxNameLen)
		}
	}

	if r.Owner != nil {
		if *r.Owner == "" {
			return errors.New("owner must not be empty")
		}
		if utf8.RuneCountInString(*r.Owner) > MaxOwnerLen {
			return fmt.Errorf("owner must be at most %d characters", MaxOwnerLen)
		}
	}

	if r.Type != nil {
		if *r.Type == "" {
			return errors.New("type must not be empty")
		}
		if utf8.RuneCountInString(*r.Type) > MaxTypeLen {
			return fmt.Errorf("type must be at most %d characters", MaxTypeLen)
		}
	}

	return nil
}

// Apply writes the fields the request carries onto the deck.
//
// The deck is modified in place: the caller reads the stored deck first, so the
// fields missing from the request are already in it.
func (r PatchDeckRequest) Apply(deck *model.Deck) {
	if r.Name != nil {
		deck.Name = *r.Name
	}
	if r.Owner != nil {
		deck.Owner = *r.Owner
	}
	if r.Type != nil {
		deck.Type = *r.Type
	}
}
