package app

import (
	"example.com/edh-stats/internal/app/model"
	"example.com/edh-stats/internal/app/storage"
)

func createPlayer() {
	var s *storage.Store
	var player *model.Player
	_, _ = s.Player().Create(player, s)
}
