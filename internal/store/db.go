package store

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
	_ "modernc.org/sqlite"
)

// Open opens the database at path and brings its schema up to date.
//
// Many processes share this file: every MCP server, the TUI, the CLI and the
// web server. The connection settings ride in the DSN so that every
// connection the pool opens gets them, not only the first:
//
//   - busy_timeout: wait for another process's lock instead of failing at once
//     with "database is locked"
//   - foreign_keys: enforce references and ON DELETE CASCADE
//   - journal_mode=WAL: a read in progress and a writer do not block each other
//
// WAL is a property of the file, so it persists, and builds from before this
// change read and write a WAL database normally. It keeps two companion files
// next to the database, "-wal" and "-shm".
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// busyTimeoutMS is how long a connection waits for another process's lock.
const busyTimeoutMS = 5000

// dsn builds the SQLite URI for path. The driver applies busy_timeout before
// the other pragmas, so the switch to WAL also waits its turn.
func dsn(path string) string {
	// In a URI filename ?, # and % have meaning and must be escaped.
	escaped := strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23").Replace(path)
	return fmt.Sprintf("file:%s?_pragma=busy_timeout(%d)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)",
		escaped, busyTimeoutMS)
}

func migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	return migrateAlter(db)
}

// migrateAlter applies additive ALTER TABLE migrations. Each statement is
// idempotent: SQLite returns "duplicate column name" when a column already
// exists, which we swallow.
func migrateAlter(db *sql.DB) error {
	alters := []string{
		`ALTER TABLE users ADD COLUMN password_hash TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE projects ADD COLUMN sync_origin TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE projects ADD COLUMN sync_cursor INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE projects ADD COLUMN sync_status TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN sync_origin TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE projects ADD COLUMN repo_dir TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE projects ADD COLUMN repo_remote TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE projects ADD COLUMN artifact_policy TEXT NOT NULL DEFAULT ''`,
	}
	for _, stmt := range alters {
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}
	return backfillRepo(db)
}

// backfillRepo sorts each old repo_path into repo_dir or repo_remote. It
// touches only projects with neither set, so running it at every start is
// harmless and a value set since is kept.
func backfillRepo(db *sql.DB) error {
	rows, err := db.Query(`SELECT id, repo_path FROM projects WHERE repo_dir = '' AND repo_remote = '' AND repo_path <> ''`)
	if err != nil {
		return err
	}
	type fill struct{ id, dir, remote string }
	var fills []fill
	for rows.Next() {
		var id, repoPath string
		if err := rows.Scan(&id, &repoPath); err != nil {
			rows.Close()
			return err
		}
		dir, remote := domain.ClassifyRepo(repoPath)
		fills = append(fills, fill{id, dir, remote})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, f := range fills {
		if _, err := db.Exec(`UPDATE projects SET repo_dir = ?, repo_remote = ? WHERE id = ? AND repo_dir = '' AND repo_remote = ''`,
			f.dir, f.remote, f.id); err != nil {
			return err
		}
	}
	return nil
}

const schema = `
CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    repo_path   TEXT NOT NULL DEFAULT '',
    sprint_mode INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sequences (
    project_id TEXT PRIMARY KEY REFERENCES projects(id),
    next_seq   INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS users (
    id           TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    is_agent     INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sprints (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id),
    name       TEXT NOT NULL,
    start_date TEXT,
    end_date   TEXT,
    status     TEXT NOT NULL DEFAULT 'planning'
);

CREATE TABLE IF NOT EXISTS tasks (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id),
    sprint_id   INTEGER REFERENCES sprints(id),
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'backlog',
    priority    TEXT NOT NULL DEFAULT 'medium',
    points      INTEGER,
    assignee_id TEXT REFERENCES users(id),
    branch      TEXT NOT NULL DEFAULT '',
    pr_number   INTEGER,
    pr_merged   INTEGER NOT NULL DEFAULT 0,
    labels      TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS task_deps (
    blocker_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    blocked_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    PRIMARY KEY (blocker_id, blocked_id)
);

CREATE TABLE IF NOT EXISTS change_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id  TEXT    NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    event_type  TEXT    NOT NULL,
    payload     TEXT    NOT NULL,
    origin      TEXT,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE INDEX IF NOT EXISTS idx_change_log_project_cursor ON change_log(project_id, id);

CREATE TABLE IF NOT EXISTS sync_secondaries (
    id              TEXT PRIMARY KEY,
    token_hash      TEXT NOT NULL,
    display_name    TEXT NOT NULL,
    last_seen_at    TEXT,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sync_secondaries_token_hash ON sync_secondaries(token_hash);

CREATE TABLE IF NOT EXISTS sync_grants (
    secondary_id TEXT NOT NULL REFERENCES sync_secondaries(id) ON DELETE CASCADE,
    project_id   TEXT NOT NULL REFERENCES projects(id)         ON DELETE CASCADE,
    PRIMARY KEY (secondary_id, project_id)
);

CREATE TABLE IF NOT EXISTS task_notes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id    TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    author_id  TEXT NOT NULL DEFAULT '',
    body       TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_task_notes_task ON task_notes(task_id, id);

CREATE TABLE IF NOT EXISTS blobs (
    sha256          TEXT PRIMARY KEY,
    size            INTEGER NOT NULL,
    in_local        INTEGER NOT NULL DEFAULT 0,
    in_remote       INTEGER NOT NULL DEFAULT 0,
    uploaded_at     TEXT,
    unreferenced_at TEXT,
    last_used_at    TEXT
);

CREATE TABLE IF NOT EXISTS artifacts (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id   TEXT NOT NULL REFERENCES projects(id),
    task_id      TEXT REFERENCES tasks(id) ON DELETE CASCADE,
    kind         TEXT NOT NULL,
    name         TEXT NOT NULL,
    media_type   TEXT NOT NULL DEFAULT '',
    sha256       TEXT NOT NULL REFERENCES blobs(sha256),
    size         INTEGER NOT NULL,
    note         TEXT NOT NULL DEFAULT '',
    origin       TEXT NOT NULL,
    source_path  TEXT NOT NULL DEFAULT '',
    source_mtime INTEGER NOT NULL DEFAULT 0,
    session_id   TEXT NOT NULL DEFAULT '',
    created_by   TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    discarded_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_artifacts_project_task ON artifacts(project_id, task_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_source_path ON artifacts(source_path);
CREATE INDEX IF NOT EXISTS idx_artifacts_sha256 ON artifacts(sha256);

CREATE TABLE IF NOT EXISTS kv (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sweep_skips (
    dir     TEXT PRIMARY KEY,
    reason  TEXT NOT NULL,
    count   INTEGER NOT NULL DEFAULT 0,
    last_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS task_links (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id    TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL,
    repo       TEXT NOT NULL DEFAULT '',
    ref        TEXT NOT NULL,
    title      TEXT NOT NULL DEFAULT '',
    url        TEXT NOT NULL DEFAULT '',
    state      TEXT NOT NULL DEFAULT '',
    source     TEXT NOT NULL DEFAULT 'manual',
    created_at TEXT NOT NULL,
    UNIQUE(task_id, kind, repo, ref)
);
CREATE INDEX IF NOT EXISTS idx_task_links_repo ON task_links(repo, kind, state);
`
