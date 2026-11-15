package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

const artifactCols = `a.id, a.project_id, a.task_id, a.kind, a.name, a.media_type, a.sha256, a.size, a.note, a.origin,
	a.source_path, a.source_mtime, a.session_id, a.created_by, a.created_at, a.updated_at, a.discarded_at,
	COALESCE(b.in_local, 0), COALESCE(b.in_remote, 0)`

const artifactFrom = ` FROM artifacts a LEFT JOIN blobs b ON b.sha256 = a.sha256`

func scanArtifact(r rowScanner) (*domain.Artifact, error) {
	var a domain.Artifact
	var taskID, discarded sql.NullString
	var kind, created, updated string
	var inLocal, inRemote int
	if err := r.Scan(&a.ID, &a.ProjectID, &taskID, &kind, &a.Name, &a.MediaType, &a.SHA256, &a.Size, &a.Note, &a.Origin,
		&a.SourcePath, &a.SourceMTime, &a.SessionID, &a.CreatedBy, &created, &updated, &discarded,
		&inLocal, &inRemote); err != nil {
		return nil, err
	}
	a.Kind = domain.ArtifactKind(kind)
	if taskID.Valid {
		a.TaskID = &taskID.String
	}
	a.CreatedAt, a.UpdatedAt = parseStamp(created), parseStamp(updated)
	if discarded.Valid {
		t := parseStamp(discarded.String)
		a.DiscardedAt = &t
	}
	switch {
	case inLocal == 1 && inRemote == 1:
		a.Where = domain.WhereBoth
	case inLocal == 1:
		a.Where = domain.WhereHere
	case inRemote == 1:
		a.Where = domain.WhereRemote
	default:
		a.Where = domain.WhereUnavailable
	}
	return &a, nil
}

// AddArtifact stores a and sets its ID. Its blob must already be recorded.
func (s *Store) AddArtifact(a *domain.Artifact) error {
	res, err := s.db.Exec(`INSERT INTO artifacts(project_id, task_id, kind, name, media_type, sha256, size, note, origin,
		source_path, source_mtime, session_id, created_by, created_at, updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		a.ProjectID, nullableStr(a.TaskID), string(a.Kind), a.Name, a.MediaType, a.SHA256, a.Size, a.Note, a.Origin,
		a.SourcePath, a.SourceMTime, a.SessionID, a.CreatedBy, stamp(a.CreatedAt), stamp(a.UpdatedAt))
	if err != nil {
		return err
	}
	a.ID, err = res.LastInsertId()
	return err
}

func (s *Store) GetArtifact(id int64) (*domain.Artifact, error) {
	a, err := scanArtifact(s.db.QueryRow(`SELECT `+artifactCols+artifactFrom+` WHERE a.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return a, err
}

func (s *Store) ListArtifacts(q domain.ArtifactQuery) ([]*domain.Artifact, error) {
	var where []string
	var args []any
	if q.TaskID != "" {
		where = append(where, "a.task_id = ?")
		args = append(args, q.TaskID)
	}
	if q.ProjectID != "" {
		where = append(where, "a.project_id = ?")
		args = append(args, q.ProjectID)
	}
	if q.Unfiled {
		where = append(where, "a.task_id IS NULL")
	}
	if !q.IncludeDiscarded {
		where = append(where, "a.discarded_at IS NULL")
	}
	query := `SELECT ` + artifactCols + artifactFrom
	if len(where) > 0 {
		query += ` WHERE ` + strings.Join(where, " AND ")
	}
	return s.queryArtifacts(query+` ORDER BY a.id`, args...)
}

func (s *Store) queryArtifacts(query string, args ...any) ([]*domain.Artifact, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Artifact
	for rows.Next() {
		a, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// LatestArtifactForPath returns the newest artifact kept from path, discarded
// or not, or nil.
func (s *Store) LatestArtifactForPath(path string) (*domain.Artifact, error) {
	a, err := scanArtifact(s.db.QueryRow(`SELECT `+artifactCols+artifactFrom+` WHERE a.source_path = ? ORDER BY a.id DESC LIMIT 1`, path))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return a, err
}

// PathDiscarded reports whether an artifact kept from path was discarded.
func (s *Store) PathDiscarded(path string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM artifacts WHERE source_path = ? AND discarded_at IS NOT NULL`, path).Scan(&n)
	return n > 0, err
}

// UpdateArtifact changes the fields p names and the update time.
func (s *Store) UpdateArtifact(id int64, p domain.ArtifactPatch) error {
	sets := []string{"updated_at = ?"}
	args := []any{stamp(time.Now().UTC())}
	set := func(col string, v any) {
		sets = append(sets, col+" = ?")
		args = append(args, v)
	}
	if p.TaskID.Set {
		set("task_id", nullableStr(p.TaskID.Value))
	}
	if p.SHA256.Set {
		set("sha256", p.SHA256.Value)
	}
	if p.Size.Set {
		set("size", p.Size.Value)
	}
	if p.SourceMTime.Set {
		set("source_mtime", p.SourceMTime.Value)
	}
	if p.Origin.Set {
		set("origin", p.Origin.Value)
	}
	if p.DiscardedAt.Set {
		var v any
		if p.DiscardedAt.Value != nil {
			v = stamp(*p.DiscardedAt.Value)
		}
		set("discarded_at", v)
	}
	args = append(args, id)
	res, err := s.db.Exec(`UPDATE artifacts SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// BlobRefs counts the artifacts, not discarded, that use sha.
func (s *Store) BlobRefs(sha string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM artifacts WHERE sha256 = ? AND discarded_at IS NULL`, sha).Scan(&n)
	return n, err
}

// --- blobs ---

const blobCols = `sha256, size, in_local, in_remote, uploaded_at, unreferenced_at, last_used_at`

func scanBlob(r rowScanner) (*domain.Blob, error) {
	var b domain.Blob
	var inLocal, inRemote int
	var uploaded, unref, used sql.NullString
	if err := r.Scan(&b.SHA256, &b.Size, &inLocal, &inRemote, &uploaded, &unref, &used); err != nil {
		return nil, err
	}
	b.InLocal, b.InRemote = inLocal == 1, inRemote == 1
	b.UploadedAt, b.UnreferencedAt, b.LastUsedAt = optStamp(uploaded), optStamp(unref), optStamp(used)
	return &b, nil
}

func optStamp(s sql.NullString) *time.Time {
	if !s.Valid {
		return nil
	}
	t := parseStamp(s.String)
	return &t
}

// PutBlob records a blob, or marks a recorded one as held here when b says
// so. A blob's size never changes: its name is its contents.
func (s *Store) PutBlob(b domain.Blob) error {
	_, err := s.db.Exec(`INSERT INTO blobs(sha256, size, in_local, in_remote) VALUES(?,?,?,?)
		ON CONFLICT(sha256) DO UPDATE SET in_local = MAX(in_local, excluded.in_local), in_remote = MAX(in_remote, excluded.in_remote)`,
		b.SHA256, b.Size, boolInt(b.InLocal), boolInt(b.InRemote))
	return err
}

func (s *Store) GetBlob(sha string) (*domain.Blob, error) {
	b, err := scanBlob(s.db.QueryRow(`SELECT `+blobCols+` FROM blobs WHERE sha256 = ?`, sha))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return b, err
}

func (s *Store) SetBlobLocal(sha string, local bool) error {
	_, err := s.db.Exec(`UPDATE blobs SET in_local = ? WHERE sha256 = ?`, boolInt(local), sha)
	return err
}

// SetBlobRemote records that the remote holds sha, confirmed at at.
func (s *Store) SetBlobRemote(sha string, at time.Time) error {
	_, err := s.db.Exec(`UPDATE blobs SET in_remote = 1, uploaded_at = ? WHERE sha256 = ?`, stamp(at), sha)
	return err
}

// ClearBlobRemote records that the remote no longer holds sha.
func (s *Store) ClearBlobRemote(sha string) error {
	_, err := s.db.Exec(`UPDATE blobs SET in_remote = 0 WHERE sha256 = ?`, sha)
	return err
}

// TouchBlob records that sha was read at at, which keeps it from eviction
// longer.
func (s *Store) TouchBlob(sha string, at time.Time) error {
	_, err := s.db.Exec(`UPDATE blobs SET last_used_at = ? WHERE sha256 = ?`, stamp(at), sha)
	return err
}

// LocalBytes is the size of everything held here.
func (s *Store) LocalBytes() (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COALESCE(SUM(size), 0) FROM blobs WHERE in_local = 1`).Scan(&n)
	return n, err
}

// MarkUnreferenced dates blobs that no artifact uses any more (a discarded
// artifact does not count) and clears the date on blobs that are used again.
func (s *Store) MarkUnreferenced(now time.Time) error {
	if _, err := s.db.Exec(`UPDATE blobs SET unreferenced_at = ? WHERE unreferenced_at IS NULL AND NOT EXISTS
		(SELECT 1 FROM artifacts a WHERE a.sha256 = blobs.sha256 AND a.discarded_at IS NULL)`, stamp(now)); err != nil {
		return err
	}
	_, err := s.db.Exec(`UPDATE blobs SET unreferenced_at = NULL WHERE unreferenced_at IS NOT NULL AND EXISTS
		(SELECT 1 FROM artifacts a WHERE a.sha256 = blobs.sha256 AND a.discarded_at IS NULL)`)
	return err
}

// EvictionCandidates lists what may be removed from this machine to make
// room, in the order to remove it: first contents nothing uses, oldest first;
// then contents the remote holds that are larger than keepLocalUnder, least
// recently used first. Contents only held here are never offered.
func (s *Store) EvictionCandidates(keepLocalUnder int64) ([]domain.Blob, error) {
	rows, err := s.db.Query(`SELECT `+blobCols+` FROM blobs WHERE in_local = 1 AND (
			unreferenced_at IS NOT NULL OR (in_remote = 1 AND size > ?))
		ORDER BY (unreferenced_at IS NULL), unreferenced_at, COALESCE(last_used_at, uploaded_at, ''), sha256`, keepLocalUnder)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Blob
	for rows.Next() {
		b, err := scanBlob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *b)
	}
	return out, rows.Err()
}

// ListBlobs returns the blobs matching a condition, for the uploader and
// housekeeping. cond is one of the BlobsXxx constants.
func (s *Store) ListBlobs(cond string, args ...any) ([]domain.Blob, error) {
	rows, err := s.db.Query(`SELECT `+blobCols+` FROM blobs WHERE `+cond+` ORDER BY sha256`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Blob
	for rows.Next() {
		b, err := scanBlob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *b)
	}
	return out, rows.Err()
}

// --- kv ---

func (s *Store) KVGet(key string) (string, bool, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM kv WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return v, err == nil, err
}

func (s *Store) KVSet(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO kv(key, value, updated_at) VALUES(?,?,?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, stamp(time.Now().UTC()))
	return err
}

// KVLease takes or renews a lease named key for holder until now+ttl. It
// succeeds when the lease is free, expired, or already holder's, and is
// atomic across processes sharing the database.
func (s *Store) KVLease(key, holder string, ttl time.Duration, now time.Time) (bool, error) {
	value := holder + "|" + stamp(now.Add(ttl))
	res, err := s.db.Exec(`INSERT INTO kv(key, value, updated_at) VALUES(?,?,?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
		WHERE substr(kv.value, 1, instr(kv.value, '|') - 1) = ? OR substr(kv.value, instr(kv.value, '|') + 1) < ?`,
		key, value, stamp(now), holder, stamp(now))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}
