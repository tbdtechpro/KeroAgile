package domain

import (
	"fmt"
	"strings"
	"time"
)

// An artifact is a file kept against a ticket: a script, a log, a screenshot,
// a recording. Its contents live outside the database, addressed by their
// SHA-256 (a Blob), so identical contents are stored once however many
// artifacts use them. An artifact with no ticket is unfiled: kept for its
// project until someone files it on a ticket or discards it.

// ArtifactKind is what sort of file an artifact is.
type ArtifactKind string

const (
	KindScript   ArtifactKind = "script"
	KindLog      ArtifactKind = "log"
	KindImage    ArtifactKind = "image"
	KindAudio    ArtifactKind = "audio"
	KindVideo    ArtifactKind = "video"
	KindDocument ArtifactKind = "document"
	KindOther    ArtifactKind = "other"
)

// Valid reports whether k is one of the seven kinds.
func (k ArtifactKind) Valid() bool {
	switch k {
	case KindScript, KindLog, KindImage, KindAudio, KindVideo, KindDocument, KindOther:
		return true
	}
	return false
}

// How an artifact came to be kept.
const (
	OriginAttach = "attach" // someone attached it to a ticket
	OriginSweep  = "sweep"  // the sweep found it in a session's scratch folder
)

// Where an artifact's contents are.
const (
	WhereHere        = "here"        // on this machine only
	WhereRemote      = "remote"      // on the remote store only
	WhereBoth        = "both"        // here and on the remote
	WhereUnavailable = "unavailable" // in neither
)

type Artifact struct {
	ID          int64        `json:"id"`
	ProjectID   string       `json:"project_id"`
	TaskID      *string      `json:"task_id"` // nil: unfiled
	Kind        ArtifactKind `json:"kind"`
	Name        string       `json:"name"`
	MediaType   string       `json:"media_type,omitempty"`
	SHA256      string       `json:"sha256"`
	Size        int64        `json:"size"`
	Note        string       `json:"note,omitempty"`
	Origin      string       `json:"origin"`
	SourcePath  string       `json:"source_path,omitempty"`
	SourceMTime int64        `json:"-"` // the source file's modification time, in nanoseconds, when last read
	SessionID   string       `json:"session_id,omitempty"`
	CreatedBy   string       `json:"created_by,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	DiscardedAt *time.Time   `json:"discarded_at,omitempty"`
	Where       string       `json:"where"` // here | remote | both | unavailable
}

// Blob is one distinct file content and where it is held.
type Blob struct {
	SHA256         string     `json:"sha256"`
	Size           int64      `json:"size"`
	InLocal        bool       `json:"in_local"`
	InRemote       bool       `json:"in_remote"`
	UploadedAt     *time.Time `json:"uploaded_at,omitempty"`
	UnreferencedAt *time.Time `json:"unreferenced_at,omitempty"` // since when no artifact uses it
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
}

// ArtifactQuery selects artifacts: a ticket's, or a project's (all of them,
// or only the unfiled ones). Discarded artifacts are left out unless asked
// for.
type ArtifactQuery struct {
	TaskID           string
	ProjectID        string
	Unfiled          bool
	IncludeDiscarded bool
}

// ArtifactPatch changes some fields of an artifact.
type ArtifactPatch struct {
	TaskID      Change[*string]
	SHA256      Change[string]
	Size        Change[int64]
	SourceMTime Change[int64]
	Origin      Change[string]
	DiscardedAt Change[*time.Time]
}

// RecordArtifact checks a and stores it. The blob it names must already be
// recorded. CreatedAt and UpdatedAt are set to now.
func (s *Service) RecordArtifact(a *Artifact) error {
	p, err := s.project(a.ProjectID)
	if err != nil {
		return err
	}
	a.ProjectID = p.ID
	if a.TaskID != nil {
		t, err := s.task(*a.TaskID)
		if err != nil {
			return err
		}
		if t.ProjectID != p.ID {
			return fmt.Errorf("%w file: %s belongs to project %s, not %s", ErrInvalid, t.ID, t.ProjectID, p.ID)
		}
		a.TaskID = &t.ID
	}
	if !a.Kind.Valid() {
		return fmt.Errorf("%w kind %q: use script, log, image, audio, video, document or other", ErrInvalid, a.Kind)
	}
	if strings.TrimSpace(a.Name) == "" || a.SHA256 == "" {
		return fmt.Errorf("%w file: a name and contents are required", ErrInvalid)
	}
	now := time.Now().UTC()
	a.CreatedAt, a.UpdatedAt = now, now
	return s.store.AddArtifact(a)
}

// GetArtifact returns one artifact, discarded or not.
func (s *Service) GetArtifact(id int64) (*Artifact, error) {
	a, err := s.store.GetArtifact(id)
	if err != nil {
		return nil, fmt.Errorf("file %d: %w", id, err)
	}
	return a, nil
}

// ListArtifacts returns the artifacts q selects, oldest first.
func (s *Service) ListArtifacts(q ArtifactQuery) ([]*Artifact, error) {
	if q.TaskID != "" {
		t, err := s.task(q.TaskID)
		if err != nil {
			return nil, err
		}
		q.TaskID = t.ID
	}
	if q.ProjectID != "" {
		p, err := s.project(q.ProjectID)
		if err != nil {
			return nil, err
		}
		q.ProjectID = p.ID
	}
	if q.TaskID == "" && q.ProjectID == "" {
		return nil, fmt.Errorf("%w files: give a ticket or a project", ErrInvalid)
	}
	return s.store.ListArtifacts(q)
}

// FileArtifact puts an artifact on a ticket of its own project. An artifact
// already on a ticket can be moved to another.
func (s *Service) FileArtifact(id int64, taskID string) (*Artifact, error) {
	a, err := s.GetArtifact(id)
	if err != nil {
		return nil, err
	}
	if a.DiscardedAt != nil {
		return nil, fmt.Errorf("%w file %d: it was discarded", ErrInvalid, id)
	}
	t, err := s.task(taskID)
	if err != nil {
		return nil, err
	}
	if t.ProjectID != a.ProjectID {
		return nil, fmt.Errorf("%w file %d: it belongs to project %s, and %s to %s", ErrInvalid, id, a.ProjectID, t.ID, t.ProjectID)
	}
	if err := s.store.UpdateArtifact(id, ArtifactPatch{TaskID: To(&t.ID)}); err != nil {
		return nil, err
	}
	return s.GetArtifact(id)
}

// DiscardArtifact hides an artifact and remembers that it was discarded, so
// the sweep does not keep the same file again. Its contents are removed later,
// once nothing else uses them.
func (s *Service) DiscardArtifact(id int64) error {
	a, err := s.GetArtifact(id)
	if err != nil {
		return err
	}
	if a.DiscardedAt != nil {
		return nil
	}
	now := time.Now().UTC()
	return s.store.UpdateArtifact(id, ArtifactPatch{DiscardedAt: To(&now)})
}

// EffectivePolicy is what happens to p's files: its own policy, or def when
// it has none. def itself falls back to keeping files locally.
func EffectivePolicy(p *Project, def ArtifactPolicy) ArtifactPolicy {
	if p != nil && p.ArtifactPolicy != PolicyDefault {
		return p.ArtifactPolicy
	}
	if def == PolicyDefault {
		return PolicyLocal
	}
	return def
}

// SweepSkip is a directory a sweep captured nothing from, and why: no
// project, or two projects equally.
type SweepSkip struct {
	Dir    string    `json:"dir"`
	Reason string    `json:"reason"`
	Count  int       `json:"count"`
	LastAt time.Time `json:"last_at"`
}
