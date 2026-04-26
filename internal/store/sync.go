package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/tbdtechpro/KeroAgile/internal/syncsrv"
)

func (s *Store) WriteChangeLog(projectID, eventType string, payload []byte, origin string) (int64, error) {
	var originVal any
	if origin != "" {
		originVal = origin
	}
	result, err := s.db.Exec(
		`INSERT INTO change_log (project_id, event_type, payload, origin) VALUES (?, ?, ?, ?)`,
		projectID, eventType, string(payload), originVal,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Store) ReadChanges(projectID string, since int64) ([]syncsrv.ChangeEvent, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, event_type, payload, COALESCE(origin,'')
         FROM change_log WHERE project_id = ? AND id > ? ORDER BY id ASC`,
		projectID, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []syncsrv.ChangeEvent
	for rows.Next() {
		var e syncsrv.ChangeEvent
		var payload string
		if err := rows.Scan(&e.Cursor, &e.ProjectID, &e.EventType, &payload, &e.Origin); err != nil {
			return nil, err
		}
		e.Payload = json.RawMessage(payload)
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *Store) AddSecondary(id, displayName string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	// Encode raw bytes as hex for a 64-char token
	token := fmt.Sprintf("%x", raw)
	hash := syncsrv.SHA256Hex(token)
	_, err := s.db.Exec(
		`INSERT INTO sync_secondaries (id, token_hash, display_name) VALUES (?, ?, ?)`,
		id, hash, displayName,
	)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *Store) ListSecondaries() ([]*syncsrv.Secondary, error) {
	rows, err := s.db.Query(
		`SELECT id, display_name, last_seen_at, created_at FROM sync_secondaries ORDER BY created_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*syncsrv.Secondary
	for rows.Next() {
		sec := &syncsrv.Secondary{}
		var lastSeen sql.NullString
		if err := rows.Scan(&sec.ID, &sec.DisplayName, &lastSeen, &sec.CreatedAt); err != nil {
			return nil, err
		}
		if lastSeen.Valid && lastSeen.String != "" {
			sec.LastSeenAt = &lastSeen.String
		}
		out = append(out, sec)
	}
	return out, rows.Err()
}

func (s *Store) GetSecondaryByTokenHash(hash string) (*syncsrv.Secondary, error) {
	sec := &syncsrv.Secondary{}
	var lastSeen sql.NullString
	err := s.db.QueryRow(
		`SELECT id, display_name, token_hash, last_seen_at, created_at
         FROM sync_secondaries WHERE token_hash = ?`, hash,
	).Scan(&sec.ID, &sec.DisplayName, &sec.TokenHash, &lastSeen, &sec.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastSeen.Valid && lastSeen.String != "" {
		sec.LastSeenAt = &lastSeen.String
	}
	return sec, nil
}

func (s *Store) RevokeSecondary(id string) error {
	_, err := s.db.Exec(`DELETE FROM sync_secondaries WHERE id = ?`, id)
	return err
}

func (s *Store) TouchSecondary(id string) error {
	_, err := s.db.Exec(
		`UPDATE sync_secondaries SET last_seen_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (s *Store) GrantProject(secondaryID, projectID string) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO sync_grants (secondary_id, project_id) VALUES (?, ?)`,
		secondaryID, projectID,
	)
	return err
}

func (s *Store) RevokeGrant(secondaryID, projectID string) error {
	_, err := s.db.Exec(
		`DELETE FROM sync_grants WHERE secondary_id = ? AND project_id = ?`,
		secondaryID, projectID,
	)
	return err
}

func (s *Store) ListGrantedProjects(secondaryID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT project_id FROM sync_grants WHERE secondary_id = ?`, secondaryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) IsGranted(secondaryID, projectID string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM sync_grants WHERE secondary_id = ? AND project_id = ?`,
		secondaryID, projectID,
	).Scan(&count)
	return count > 0, err
}

func (s *Store) SetProjectSyncCursor(projectID string, cursor int64) error {
	_, err := s.db.Exec(`UPDATE projects SET sync_cursor = ? WHERE id = ?`, cursor, projectID)
	return err
}

func (s *Store) SetProjectSyncStatus(projectID string, status string) error {
	_, err := s.db.Exec(`UPDATE projects SET sync_status = ? WHERE id = ?`, status, projectID)
	return err
}

// SetSyncOrigin sets the sync_origin for a project. Used in tests and by the sync/join endpoint.
func (s *Store) SetSyncOrigin(projectID, origin string) error {
	_, err := s.db.Exec(`UPDATE projects SET sync_origin = ? WHERE id = ?`, origin, projectID)
	return err
}
