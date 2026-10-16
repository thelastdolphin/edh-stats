// Package getstats implements the statistics endpoints.
package getstats

import (
	"log"
	"net/http"

	httpUtils "github.com/thelastdolphin/edh-stats/internal/app/http"
	"github.com/thelastdolphin/edh-stats/internal/app/storage"
)

// DeckStats handles GET /stats/decks.
//
// One row per deck that took part in a game, as a plain array like the other
// listings. The optional from and to parameters narrow the period inclusively.
func DeckStats(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		from, err := httpUtils.ParseQueryDate(q.Get("from"))
		if err != nil {
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeInvalidQuery,
				"from must be a date in YYYY-MM-DD form")
			return
		}

		to, err := httpUtils.ParseQueryDate(q.Get("to"))
		if err != nil {
			httpUtils.Error(w, http.StatusBadRequest, httpUtils.CodeInvalidQuery,
				"to must be a date in YYYY-MM-DD form")
			return
		}

		stats, err := store.Game().DeckStats(r.Context(), storage.StatsFilters{From: from, To: to})
		if err != nil {
			log.Printf("deck statistics: %v", err)
			httpUtils.Error(w, http.StatusInternalServerError, httpUtils.CodeInternal, "internal error")
			return
		}
		httpUtils.JSONEncode(w, http.StatusOK, stats)
	}
}
