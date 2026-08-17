package auth

import "database/sql"

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) DB() *sql.DB {
	return s.db
}
