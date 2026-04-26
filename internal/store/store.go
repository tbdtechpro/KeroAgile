package store

import (
	"database/sql"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/syncsrv"
)

// Store implements domain.Store using SQLite.
type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// compile-time interface checks
var _ domain.Store = (*Store)(nil)
var _ syncsrv.PrimaryStore = (*Store)(nil)
var _ syncsrv.SecondaryStore = (*Store)(nil)

func (s *Store) Close() error {
	return s.db.Close()
}
