package store

import (
	"time"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// AddNote stores n and sets its ID.
func (s *Store) AddNote(n *domain.Note) error {
	res, err := s.db.Exec(`INSERT INTO task_notes(task_id, author_id, body, created_at) VALUES(?,?,?,?)`,
		n.TaskID, n.AuthorID, n.Body, stamp(n.CreatedAt))
	if err != nil {
		return err
	}
	n.ID, err = res.LastInsertId()
	return err
}

// ListNotes returns a ticket's notes, oldest first.
func (s *Store) ListNotes(taskID string) ([]*domain.Note, error) {
	rows, err := s.db.Query(`SELECT id, task_id, author_id, body, created_at FROM task_notes WHERE task_id = ? ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Note
	for rows.Next() {
		var n domain.Note
		var created string
		if err := rows.Scan(&n.ID, &n.TaskID, &n.AuthorID, &n.Body, &created); err != nil {
			return nil, err
		}
		n.CreatedAt = parseStamp(created)
		out = append(out, &n)
	}
	return out, rows.Err()
}

// parseStamp reads a time written by stamp, or an older whole-second one.
func parseStamp(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
