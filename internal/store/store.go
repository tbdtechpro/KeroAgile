package store

import (
	"database/sql"

	"keroagile/internal/domain"
)

// Store implements domain.Store using SQLite.
type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// compile-time interface check
var _ domain.Store = (*Store)(nil)

func (s *Store) Close() error {
	return s.db.Close()
}
