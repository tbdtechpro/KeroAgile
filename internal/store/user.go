package store

import (
	"database/sql"
	"errors"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

func (s *Store) CreateUser(u *domain.User) error {
	_, err := s.db.Exec(
		`INSERT INTO users(id, display_name, is_agent, password_hash) VALUES(?,?,?,?)`,
		u.ID, u.DisplayName, boolInt(u.IsAgent), u.PasswordHash,
	)
	return err
}

func (s *Store) ListUsers() ([]*domain.User, error) {
	rows, err := s.db.Query(`SELECT id, display_name, is_agent, password_hash FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) GetUser(id string) (*domain.User, error) {
	row := s.db.QueryRow(`SELECT id, display_name, is_agent, password_hash FROM users WHERE id=?`, id)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return u, err
}

func (s *Store) SetUserPassword(id, hash string) error {
	res, err := s.db.Exec(`UPDATE users SET password_hash=? WHERE id=?`, hash, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanUser(r rowScanner) (*domain.User, error) {
	var u domain.User
	var isAgent int
	if err := r.Scan(&u.ID, &u.DisplayName, &isAgent, &u.PasswordHash); err != nil {
		return nil, err
	}
	u.IsAgent = isAgent == 1
	return &u, nil
}
