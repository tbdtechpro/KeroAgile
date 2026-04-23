package store

import (
	"database/sql"
	"errors"
	"time"

	"keroagile/internal/domain"
)

func (s *Store) CreateSprint(sp *domain.Sprint) (*domain.Sprint, error) {
	res, err := s.db.Exec(
		`INSERT INTO sprints(project_id, name, start_date, end_date, status) VALUES(?,?,?,?,?)`,
		sp.ProjectID, sp.Name, nullTime(sp.StartDate), nullTime(sp.EndDate), string(sp.Status),
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	sp.ID = id
	return sp, nil
}

func (s *Store) ListSprints(projectID string) ([]*domain.Sprint, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, name, start_date, end_date, status FROM sprints WHERE project_id=? ORDER BY id`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Sprint
	for rows.Next() {
		sp, err := scanSprint(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

func (s *Store) GetSprint(id int64) (*domain.Sprint, error) {
	row := s.db.QueryRow(
		`SELECT id, project_id, name, start_date, end_date, status FROM sprints WHERE id=?`, id,
	)
	sp, err := scanSprint(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return sp, err
}

func (s *Store) UpdateSprint(sp *domain.Sprint) error {
	_, err := s.db.Exec(
		`UPDATE sprints SET name=?, start_date=?, end_date=?, status=? WHERE id=?`,
		sp.Name, nullTime(sp.StartDate), nullTime(sp.EndDate), string(sp.Status), sp.ID,
	)
	return err
}

func scanSprint(r rowScanner) (*domain.Sprint, error) {
	var sp domain.Sprint
	var startDate, endDate sql.NullString
	var status string
	if err := r.Scan(&sp.ID, &sp.ProjectID, &sp.Name, &startDate, &endDate, &status); err != nil {
		return nil, err
	}
	sp.Status = domain.SprintStatus(status)
	if startDate.Valid {
		t, _ := time.Parse(time.RFC3339, startDate.String)
		sp.StartDate = &t
	}
	if endDate.Valid {
		t, _ := time.Parse(time.RFC3339, endDate.String)
		sp.EndDate = &t
	}
	return &sp, nil
}

func nullTime(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.Format(time.RFC3339), Valid: true}
}
