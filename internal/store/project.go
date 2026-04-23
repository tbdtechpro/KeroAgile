package store

import (
	"database/sql"
	"errors"

	"keroagile/internal/domain"
)

func (s *Store) CreateProject(p *domain.Project) error {
	_, err := s.db.Exec(
		`INSERT INTO projects(id, name, repo_path, sprint_mode) VALUES(?,?,?,?)`,
		p.ID, p.Name, p.RepoPath, boolInt(p.SprintMode),
	)
	return err
}

func (s *Store) ListProjects() ([]*domain.Project, error) {
	rows, err := s.db.Query(`SELECT id, name, repo_path, sprint_mode FROM projects ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetProject(id string) (*domain.Project, error) {
	row := s.db.QueryRow(`SELECT id, name, repo_path, sprint_mode FROM projects WHERE id=?`, id)
	p, err := scanProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return p, err
}

func (s *Store) UpdateProject(p *domain.Project) error {
	_, err := s.db.Exec(
		`UPDATE projects SET name=?, repo_path=?, sprint_mode=? WHERE id=?`,
		p.Name, p.RepoPath, boolInt(p.SprintMode), p.ID,
	)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProject(r rowScanner) (*domain.Project, error) {
	var p domain.Project
	var sprintMode int
	err := r.Scan(&p.ID, &p.Name, &p.RepoPath, &sprintMode)
	if err != nil {
		return nil, err
	}
	p.SprintMode = sprintMode == 1
	return &p, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
