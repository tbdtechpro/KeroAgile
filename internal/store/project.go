package store

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

func (s *Store) CreateProject(p *domain.Project) error {
	_, err := s.db.Exec(
		`INSERT INTO projects(id, name, repo_path, sprint_mode, sync_origin, sync_cursor, sync_status, repo_dir, repo_remote, artifact_policy) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		p.ID, p.Name, p.RepoPath, boolInt(p.SprintMode), p.SyncOrigin, p.SyncCursor, p.SyncStatus,
		p.RepoDir, p.RepoRemote, string(p.ArtifactPolicy),
	)
	return err
}

func (s *Store) ListProjects() ([]*domain.Project, error) {
	rows, err := s.db.Query(`SELECT id, name, repo_path, sprint_mode, sync_origin, sync_cursor, sync_status, repo_dir, repo_remote, artifact_policy FROM projects ORDER BY id`)
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
	row := s.db.QueryRow(`SELECT id, name, repo_path, sprint_mode, sync_origin, sync_cursor, sync_status, repo_dir, repo_remote, artifact_policy FROM projects WHERE id=?`, id)
	p, err := scanProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return p, err
}

func (s *Store) UpdateProject(p *domain.Project) error {
	_, err := s.db.Exec(
		`UPDATE projects SET name=?, repo_path=?, sprint_mode=?, sync_origin=?, sync_cursor=?, sync_status=?,
		 repo_dir=?, repo_remote=?, artifact_policy=? WHERE id=?`,
		p.Name, p.RepoPath, boolInt(p.SprintMode), p.SyncOrigin, p.SyncCursor, p.SyncStatus,
		p.RepoDir, p.RepoRemote, string(p.ArtifactPolicy), p.ID,
	)
	return err
}

// PatchProject changes only the settings p names, in one statement.
func (s *Store) PatchProject(id string, p domain.ProjectPatch) error {
	var sets []string
	var args []any
	set := func(col string, v any) {
		sets = append(sets, col+" = ?")
		args = append(args, v)
	}
	if p.RepoDir.Set {
		set("repo_dir", p.RepoDir.Value)
	}
	if p.RepoRemote.Set {
		set("repo_remote", p.RepoRemote.Value)
	}
	if p.ArtifactPolicy.Set {
		set("artifact_policy", string(p.ArtifactPolicy.Value))
	}
	if p.SprintMode.Set {
		set("sprint_mode", boolInt(p.SprintMode.Value))
	}
	if len(sets) == 0 {
		_, err := s.GetProject(id)
		return err
	}
	args = append(args, id)
	res, err := s.db.Exec(`UPDATE projects SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProject(r rowScanner) (*domain.Project, error) {
	var p domain.Project
	var sprintMode int
	var policy string
	err := r.Scan(&p.ID, &p.Name, &p.RepoPath, &sprintMode, &p.SyncOrigin, &p.SyncCursor, &p.SyncStatus,
		&p.RepoDir, &p.RepoRemote, &policy)
	p.ArtifactPolicy = domain.ArtifactPolicy(policy)
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
