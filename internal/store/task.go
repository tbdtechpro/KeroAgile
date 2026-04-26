package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

func (s *Store) NextTaskSeq(projectID string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO sequences(project_id, next_seq) VALUES(?,1)
		 ON CONFLICT(project_id) DO UPDATE SET next_seq = next_seq + 1`,
		projectID,
	)
	if err != nil {
		return 0, err
	}

	var seq int
	if err = tx.QueryRow(`SELECT next_seq FROM sequences WHERE project_id=?`, projectID).Scan(&seq); err != nil {
		return 0, err
	}
	return seq, tx.Commit()
}

func (s *Store) CreateTask(t *domain.Task) error {
	_, err := s.db.Exec(
		`INSERT INTO tasks(id,project_id,sprint_id,title,description,status,priority,
		 points,assignee_id,branch,pr_number,pr_merged,labels,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.ProjectID, t.SprintID, t.Title, t.Description,
		string(t.Status), string(t.Priority),
		nullableInt(t.Points), nullableStr(t.AssigneeID), t.Branch,
		nullableInt(t.PRNumber), boolInt(t.PRMerged),
		strings.Join(t.Labels, ","),
		t.CreatedAt.UTC().Format(time.RFC3339),
		t.UpdatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

func (s *Store) GetTask(id string) (*domain.Task, error) {
	row := s.db.QueryRow(
		`SELECT id,project_id,sprint_id,title,description,status,priority,
		 points,assignee_id,branch,pr_number,pr_merged,labels,created_at,updated_at
		 FROM tasks WHERE id=?`, id,
	)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return t, err
}

func (s *Store) ListTasks(projectID string, f domain.TaskFilters) ([]*domain.Task, error) {
	q := `SELECT id,project_id,sprint_id,title,description,status,priority,
	      points,assignee_id,branch,pr_number,pr_merged,labels,created_at,updated_at
	      FROM tasks WHERE project_id=?`
	args := []any{projectID}

	if f.Status != nil {
		q += ` AND status=?`
		args = append(args, string(*f.Status))
	}
	if f.AssigneeID != nil {
		q += ` AND assignee_id=?`
		args = append(args, *f.AssigneeID)
	}
	if f.SprintID != nil {
		q += ` AND sprint_id=?`
		args = append(args, *f.SprintID)
	}
	q += ` ORDER BY created_at ASC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) UpdateTask(t *domain.Task) error {
	res, err := s.db.Exec(
		`UPDATE tasks SET sprint_id=?,title=?,description=?,status=?,priority=?,
		 points=?,assignee_id=?,branch=?,pr_number=?,pr_merged=?,labels=?,updated_at=?
		 WHERE id=?`,
		t.SprintID, t.Title, t.Description, string(t.Status), string(t.Priority),
		nullableInt(t.Points), nullableStr(t.AssigneeID), t.Branch,
		nullableInt(t.PRNumber), boolInt(t.PRMerged),
		strings.Join(t.Labels, ","),
		t.UpdatedAt.UTC().Format(time.RFC3339),
		t.ID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteTask(id string) error {
	_, err := s.db.Exec(`DELETE FROM tasks WHERE id=?`, id)
	return err
}

func (s *Store) GetTaskDeps(taskID string) (blockers, blocking []string, err error) {
	rows, err := s.db.Query(`SELECT blocker_id FROM task_deps WHERE blocked_id=?`, taskID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, nil, err
		}
		blockers = append(blockers, id)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	rows2, err := s.db.Query(`SELECT blocked_id FROM task_deps WHERE blocker_id=?`, taskID)
	if err != nil {
		return nil, nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var id string
		if err := rows2.Scan(&id); err != nil {
			return nil, nil, err
		}
		blocking = append(blocking, id)
	}
	return blockers, blocking, rows2.Err()
}

func (s *Store) AddDep(blockerID, blockedID string) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO task_deps(blocker_id,blocked_id) VALUES(?,?)`,
		blockerID, blockedID,
	)
	return err
}

func (s *Store) RemoveDep(blockerID, blockedID string) error {
	_, err := s.db.Exec(
		`DELETE FROM task_deps WHERE blocker_id=? AND blocked_id=?`,
		blockerID, blockedID,
	)
	return err
}

func (s *Store) SearchTasks(q string, limit int) ([]*domain.TaskSummary, error) {
	return s.SearchTasksWithHint(q, limit, "")
}

func (s *Store) SearchTasksWithHint(q string, limit int, hintProjectID string) ([]*domain.TaskSummary, error) {
	return nil, nil // stub — implemented in Task 2
}

func scanTask(r rowScanner) (*domain.Task, error) {
	var t domain.Task
	var status, priority, labels, createdAt, updatedAt string
	var points, prNumber sql.NullInt64
	var sprintID sql.NullInt64
	var assigneeID sql.NullString
	var prMerged int

	err := r.Scan(
		&t.ID, &t.ProjectID, &sprintID, &t.Title, &t.Description,
		&status, &priority, &points, &assigneeID, &t.Branch,
		&prNumber, &prMerged, &labels, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	t.Status = domain.Status(status)
	t.Priority = domain.Priority(priority)
	t.PRMerged = prMerged == 1
	if labels != "" {
		t.Labels = strings.Split(labels, ",")
	}
	if points.Valid {
		v := int(points.Int64)
		t.Points = &v
	}
	if sprintID.Valid {
		v := sprintID.Int64
		t.SprintID = &v
	}
	if assigneeID.Valid {
		t.AssigneeID = &assigneeID.String
	}
	if prNumber.Valid {
		v := int(prNumber.Int64)
		t.PRNumber = &v
	}
	if t.CreatedAt, err = time.Parse(time.RFC3339, createdAt); err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	if t.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt); err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}
	return &t, nil
}

func nullableInt(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

func nullableStr(v *string) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
}
