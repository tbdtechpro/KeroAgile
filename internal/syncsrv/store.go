package syncsrv

// PrimaryStore is the persistence contract for primary-side sync operations.
// *store.Store implements this.
type PrimaryStore interface {
	WriteChangeLog(projectID, eventType string, payload []byte, origin string) (int64, error)
	ReadChanges(projectID string, since int64) ([]ChangeEvent, error)
	AddSecondary(id, displayName string) (token string, err error)
	ListSecondaries() ([]*Secondary, error)
	GetSecondaryByTokenHash(hash string) (*Secondary, error)
	RevokeSecondary(id string) error
	TouchSecondary(id string) error
	GrantProject(secondaryID, projectID string) error
	RevokeGrant(secondaryID, projectID string) error
	ListGrantedProjects(secondaryID string) ([]string, error)
	IsGranted(secondaryID, projectID string) (bool, error)
}

// SecondaryStore is the persistence contract for secondary-side sync state.
// *store.Store implements this.
type SecondaryStore interface {
	SetProjectSyncCursor(projectID string, cursor int64) error
	SetProjectSyncStatus(projectID string, status string) error
	SetSyncOrigin(projectID, origin string) error
}
