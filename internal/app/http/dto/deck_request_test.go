package dto

import (
	"strings"
	"testing"
)

func TestCreateDeckRequest_Normalize(t *testing.T) {
	request := CreateDeckRequest{
		Name:  "  My Deck \n",
		Owner: "\t tester ",
		Type:  " control  ",
	}

	request.Normalize()

	if request.Name != "My Deck" || request.Owner != "tester" || request.Type != "control" {
		t.Errorf("Normalize() = %+v, want trimmed values", request)
	}
}

func TestCreateDeckRequest_Validate(t *testing.T) {
	longName := strings.Repeat("a", MaxNameLen+1)
	maxName := strings.Repeat("a", MaxNameLen)
	// Runes, not bytes: a 50-rune Cyrillic name is a valid 50-character value.
	maxCyrillic := strings.Repeat("щ", MaxNameLen)

	tests := []struct {
		name    string
		request CreateDeckRequest
		wantErr string
	}{
		{
			name:    "valid",
			request: CreateDeckRequest{Name: "deck", Owner: "tester", Type: "control"},
		},
		{
			name:    "max length name",
			request: CreateDeckRequest{Name: maxName, Owner: "tester", Type: "control"},
		},
		{
			name:    "max length multibyte name",
			request: CreateDeckRequest{Name: maxCyrillic, Owner: "tester", Type: "control"},
		},
		{
			name:    "name is required",
			request: CreateDeckRequest{Owner: "tester", Type: "control"},
			wantErr: "name is required",
		},
		{
			name:    "name too long",
			request: CreateDeckRequest{Name: longName, Owner: "tester", Type: "control"},
			wantErr: "name must be at most 50 characters",
		},
		{
			name:    "owner is required",
			request: CreateDeckRequest{Name: "deck", Type: "control"},
			wantErr: "owner is required",
		},
		{
			name:    "type is required",
			request: CreateDeckRequest{Name: "deck", Owner: "tester"},
			wantErr: "type is required",
		},
		{
			name:    "type too long",
			request: CreateDeckRequest{Name: "deck", Owner: "tester", Type: longName},
			wantErr: "type must be at most 50 characters",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.request.Validate()
			switch {
			case test.wantErr == "" && err != nil:
				t.Errorf("Validate() = %v, want nil", err)
			case test.wantErr != "" && err == nil:
				t.Errorf("Validate() = nil, want %q", test.wantErr)
			case test.wantErr != "" && err.Error() != test.wantErr:
				t.Errorf("Validate() = %q, want %q", err, test.wantErr)
			}
		})
	}
}
