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
	if err != nil {
		return nil, err
	}

	blockers, blocking, err := s.GetTaskDeps(id)
	if err != nil {
		return nil, err
	}
	t.Blockers = blockers
	t.Blocking = blocking

	t.BlockerDetails, err = s.fetchSummaries(blockers)
	if err != nil {
		return nil, err
	}
	t.BlockingDetails, err = s.fetchSummaries(blocking)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Store) TaskByBranch(branch string) (*domain.Task, error) {
	if branch == "" {
		return nil, domain.ErrNotFound
	}
	row := s.db.QueryRow(
		`SELECT id,project_id,sprint_id,title,description,status,priority,
		 points,assignee_id,branch,pr_number,pr_merged,labels,created_at,updated_at
		 FROM tasks WHERE branch=? ORDER BY updated_at DESC LIMIT 1`, branch,
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
	// Only return blockers whose status is not 'done'; a done task is no longer an active block.
	rows, err := s.db.Query(
		`SELECT d.blocker_id FROM task_deps d
		 JOIN tasks t ON t.id = d.blocker_id
		 WHERE d.blocked_id = ? AND t.status != 'done'`, taskID)
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
	like := "%" + q + "%"
	rows, err := s.db.Query(
		`SELECT id, title, project_id, status FROM tasks
		 WHERE id LIKE ? OR title LIKE ?
		 ORDER BY (project_id = ?) DESC, rowid ASC
		 LIMIT ?`,
		like, like, hintProjectID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.TaskSummary
	for rows.Next() {
		var ts domain.TaskSummary
		var status string
		if err := rows.Scan(&ts.ID, &ts.Title, &ts.ProjectID, &status); err != nil {
			return nil, err
		}
		ts.Status = domain.Status(status)
		out = append(out, &ts)
	}
	return out, rows.Err()
}

// fetchSummaries returns a TaskSummary for each id in ids, preserving input order.
// IDs not found in the database are silently skipped.
func (s *Store) fetchSummaries(ids []string) ([]*domain.TaskSummary, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(
		`SELECT id, title, project_id, status FROM tasks WHERE id IN (`+placeholders+`) ORDER BY rowid ASC`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byID := make(map[string]*domain.TaskSummary)
	for rows.Next() {
		var ts domain.TaskSummary
		var status string
		if err := rows.Scan(&ts.ID, &ts.Title, &ts.ProjectID, &status); err != nil {
			return nil, err
		}
		ts.Status = domain.Status(status)
		byID[ts.ID] = &ts
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]*domain.TaskSummary, 0, len(ids))
	for _, id := range ids {
		if ts, ok := byID[id]; ok {
			out = append(out, ts)
		}
	}
	return out, nil
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

// taskColumns lists a ticket's columns in the order scanTask reads them.
const taskColumns = `id,project_id,sprint_id,title,description,status,priority,
	 points,assignee_id,branch,pr_number,pr_merged,labels,created_at,updated_at`

// likeEscaper makes text literal inside a LIKE pattern whose escape character
// is a backslash, so a search for "100%" does not treat % as a wildcard.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// FindTasks returns the tickets matching q, at most q.Limit of them, and how
// many match in all. Every filter is optional and they combine with AND.
func (s *Store) FindTasks(q domain.TaskQuery) ([]*domain.Task, int, error) {
	var where []string
	var args []any
	contains := "%" + likeEscaper.Replace(q.Text) + "%"

	if q.Text != "" {
		where = append(where, `(id LIKE ? ESCAPE '\' OR title LIKE ? ESCAPE '\' OR description LIKE ? ESCAPE '\')`)
		args = append(args, contains, contains, contains)
	}
	if q.ProjectID != "" {
		where = append(where, `project_id = ?`)
		args = append(args, q.ProjectID)
	}
	if len(q.Statuses) > 0 {
		marks := strings.TrimSuffix(strings.Repeat("?,", len(q.Statuses)), ",")
		where = append(where, `status IN (`+marks+`)`)
		for _, st := range q.Statuses {
			args = append(args, string(st))
		}
	}
	if q.AssigneeID != "" {
		where = append(where, `assignee_id = ?`)
		args = append(args, q.AssigneeID)
	}
	if q.Label != "" {
		// Labels are stored joined by commas. Wrapping both sides in commas
		// matches whole labels only ("tui" is not found in "tuist"); the
		// replace tolerates labels entered as "a, b".
		where = append(where, `(',' || replace(labels, ', ', ',') || ',') LIKE ? ESCAPE '\'`)
		args = append(args, "%,"+likeEscaper.Replace(q.Label)+",%")
	}
	if q.SprintID != nil {
		where = append(where, `sprint_id = ?`)
		args = append(args, *q.SprintID)
	}
	cond := "1=1"
	if len(where) > 0 {
		cond = strings.Join(where, " AND ")
	}

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// A text search ranks an exact ID first, then title matches, then
	// description matches, most recently changed first. Without text, tickets
	// come grouped by project, oldest first, as a board lists them.
	order := `project_id, created_at, rowid`
	if q.Text != "" {
		order = `CASE WHEN id = ? COLLATE NOCASE THEN 0 WHEN title LIKE ? ESCAPE '\' THEN 1 ELSE 2 END, updated_at DESC, rowid DESC`
		args = append(args, q.Text, contains)
	}
	query := `SELECT ` + taskColumns + ` FROM tasks WHERE ` + cond + ` ORDER BY ` + order
	if q.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, q.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, t)
	}
	return out, total, rows.Err()
}

// TaskCounts returns the number of tickets per project and status. A project
// with no tickets has no entry.
func (s *Store) TaskCounts() (map[string]map[domain.Status]int, error) {
	rows, err := s.db.Query(`SELECT project_id, status, COUNT(*) FROM tasks GROUP BY project_id, status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]map[domain.Status]int)
	for rows.Next() {
		var project, status string
		var n int
		if err := rows.Scan(&project, &status, &n); err != nil {
			return nil, err
		}
		if out[project] == nil {
			out[project] = make(map[domain.Status]int)
		}
		out[project][domain.Status(status)] = n
	}
	return out, rows.Err()
}
