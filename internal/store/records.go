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

const linkCols = `id, task_id, kind, repo, ref, title, url, state, source, created_at`

func scanLink(r rowScanner) (*domain.Link, error) {
	var l domain.Link
	var kind, created string
	if err := r.Scan(&l.ID, &l.TaskID, &kind, &l.Repo, &l.Ref, &l.Title, &l.URL, &l.State, &l.Source, &created); err != nil {
		return nil, err
	}
	l.Kind = domain.LinkKind(kind)
	l.CreatedAt = parseStamp(created)
	return &l, nil
}

// UpsertLink records l, or refreshes the non-empty title, URL and state of
// the link with the same ticket, kind, repo and ref. It fills l from the
// stored row.
func (s *Store) UpsertLink(l *domain.Link) (bool, error) {
	res, err := s.db.Exec(`INSERT INTO task_links(task_id, kind, repo, ref, title, url, state, source, created_at)
		VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(task_id, kind, repo, ref) DO NOTHING`,
		l.TaskID, string(l.Kind), l.Repo, l.Ref, l.Title, l.URL, l.State, l.Source, stamp(l.CreatedAt))
	if err != nil {
		return false, err
	}
	created, _ := res.RowsAffected()
	if created == 0 {
		if _, err := s.db.Exec(`UPDATE task_links SET
				title = CASE WHEN ? <> '' THEN ? ELSE title END,
				url   = CASE WHEN ? <> '' THEN ? ELSE url END,
				state = CASE WHEN ? <> '' THEN ? ELSE state END
			WHERE task_id = ? AND kind = ? AND repo = ? AND ref = ?`,
			l.Title, l.Title, l.URL, l.URL, l.State, l.State, l.TaskID, string(l.Kind), l.Repo, l.Ref); err != nil {
			return false, err
		}
	}
	got, err := scanLink(s.db.QueryRow(`SELECT `+linkCols+` FROM task_links WHERE task_id = ? AND kind = ? AND repo = ? AND ref = ?`,
		l.TaskID, string(l.Kind), l.Repo, l.Ref))
	if err != nil {
		return false, err
	}
	*l = *got
	return created == 1, nil
}

func (s *Store) queryLinks(query string, args ...any) ([]*domain.Link, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Link
	for rows.Next() {
		l, err := scanLink(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// ListLinks returns a ticket's links, oldest first.
func (s *Store) ListLinks(taskID string) ([]*domain.Link, error) {
	return s.queryLinks(`SELECT `+linkCols+` FROM task_links WHERE task_id = ? ORDER BY id`, taskID)
}

// LinksByRepo returns the links of one kind and state to one repository.
func (s *Store) LinksByRepo(repo string, kind domain.LinkKind, state string) ([]*domain.Link, error) {
	return s.queryLinks(`SELECT `+linkCols+` FROM task_links WHERE repo = ? AND kind = ? AND state = ? ORDER BY id`,
		repo, string(kind), state)
}

// SetLinkState sets one link's state.
func (s *Store) SetLinkState(id int64, state string) error {
	_, err := s.db.Exec(`UPDATE task_links SET state = ? WHERE id = ?`, state, id)
	return err
}
