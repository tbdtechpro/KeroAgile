package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// A ticket's records are what accumulates around it while it is worked:
// notes, links to commits and pull requests, and preserved files. They are
// kept in the local database only; the cross-network sync does not carry
// them (see KA-072).

// Note is a piece of text added to a ticket. Notes are only ever added, never
// edited, and are shown oldest first, so several writers add to them without
// overwriting each other.
type Note struct {
	ID        int64     `json:"id"`
	TaskID    string    `json:"task_id"`
	AuthorID  string    `json:"author_id,omitempty"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// MaxNoteLen is the longest note accepted, in characters.
const MaxNoteLen = 20000

// AddNote adds a note to a ticket, signed by authorID (a user, or empty for
// an unsigned note).
func (s *Service) AddNote(taskID, authorID, body string) (*Note, error) {
	t, err := s.task(taskID)
	if err != nil {
		return nil, err
	}
	body = strings.TrimSpace(StripControl(body))
	if body == "" {
		return nil, fmt.Errorf("%w note: the note is empty", ErrInvalid)
	}
	if n := utf8.RuneCountInString(body); n > MaxNoteLen {
		return nil, fmt.Errorf("%w note: %d characters, at most %d", ErrInvalid, n, MaxNoteLen)
	}
	authorID = strings.TrimSpace(authorID)
	if err := s.checkAssignee(authorID); err != nil {
		return nil, fmt.Errorf("author: %w", err)
	}
	n := &Note{TaskID: t.ID, AuthorID: authorID, Body: body, CreatedAt: time.Now().UTC()}
	if err := s.store.AddNote(n); err != nil {
		return nil, err
	}
	return n, nil
}

// ListNotes returns a ticket's notes, oldest first.
func (s *Service) ListNotes(taskID string) ([]*Note, error) {
	t, err := s.task(taskID)
	if err != nil {
		return nil, err
	}
	return s.store.ListNotes(t.ID)
}

// withRecords fills in t's notes, links and preserved files.
func (s *Service) withRecords(t *Task) error {
	notes, err := s.store.ListNotes(t.ID)
	if err != nil {
		return err
	}
	t.Notes = notes
	return nil
}

// StripControl removes control characters other than newline and tab, and
// turns CRLF into LF. Text kept on the board is shown in terminals, where an
// escape sequence would be obeyed rather than shown.
func StripControl(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return -1
		}
		return r
	}, s)
}
