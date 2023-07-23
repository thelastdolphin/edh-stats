package storage

import (
	"context"
	"example.com/edh-stats/internal/app/model"
)

type PlayerRepository struct{}

func (pr *PlayerRepository) Create(p *model.Player, s *Store) (*model.Player, error) {
	if err := s.db.QueryRow(
		context.Background(),
		"INSERT INTO players (playerName) VALUES ($1) RETURNING id",
		p.PlayerName,
	).Scan(&p.ID); err != nil {
		return nil, err
	}
	return p, nil
}
