package http

import (
	"strconv"
	"time"

	"github.com/thelastdolphin/edh-stats/internal/app/model"
)

// ParseQueryInt parses an optional integer query parameter, falling back to
// defaultValue when the parameter is absent.
//
// A malformed value is reported to the caller instead of being silently replaced
// by the default: quietly ignoring "limit=abc" hides client bugs.
func ParseQueryInt(raw string, defaultValue int) (int, error) {
	if raw == "" {
		return defaultValue, nil
	}
	return strconv.Atoi(raw)
}

// ParseQueryDate validates an optional date query parameter and returns it in the
// canonical form, so that a client sending "2026-2-7" is answered with an error
// rather than with an empty result.
func ParseQueryDate(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}

	parsed, err := time.Parse(model.DateLayout, raw)
	if err != nil {
		return "", err
	}
	return parsed.Format(model.DateLayout), nil
}
