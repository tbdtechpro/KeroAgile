package store

import (
	"database/sql"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
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
	}
	for _, stmt := range alters {
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
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

CREATE TABLE IF NOT EXISTS sync_grants (
    secondary_id TEXT NOT NULL REFERENCES sync_secondaries(id) ON DELETE CASCADE,
    project_id   TEXT NOT NULL REFERENCES projects(id)         ON DELETE CASCADE,
    PRIMARY KEY (secondary_id, project_id)
);
`
