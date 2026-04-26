package syncsrv

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// Daemon orchestrates a secondary's background sync work.
type Daemon struct {
	client   *Client
	store    SecondaryStore // for SetProjectSyncCursor, SetProjectSyncStatus
	rawStore domain.Store   // for task/sprint/user upserts
	cancel   context.CancelFunc
}

// NewDaemon creates a Daemon. client is the shared Client instance (caller is responsible
// for calling client.Start() before or after NewDaemon — the daemon does not start it).
// st must implement SecondaryStore; rawStore is for entity upserts.
// Passing the same *store.Store for both is fine — it implements all needed interfaces.
func NewDaemon(client *Client, st SecondaryStore, rawStore domain.Store) *Daemon {
	return &Daemon{
		client:   client,
		store:    st,
		rawStore: rawStore,
	}
}

// InitialSync fetches a snapshot from the primary and applies all entities locally.
// Call this before Start() to seed the secondary's local store.
func (d *Daemon) InitialSync(ctx context.Context, projectIDs []string) error {
	snap, err := d.client.FetchSnapshot(ctx, projectIDs)
	if err != nil {
		return err
	}
	for _, p := range snap.Projects {
		existing, _ := d.rawStore.GetProject(p.ID)
		if existing == nil {
			if err := d.rawStore.CreateProject(p); err != nil {
				return fmt.Errorf("create project %s: %w", p.ID, err)
			}
		} else {
			if err := d.rawStore.UpdateProject(p); err != nil {
				return fmt.Errorf("update project %s: %w", p.ID, err)
			}
		}
	}
	for _, u := range snap.Users {
		existing, _ := d.rawStore.GetUser(u.ID)
		if existing == nil {
			if err := d.rawStore.CreateUser(u); err != nil {
				return fmt.Errorf("create user %s: %w", u.ID, err)
			}
		}
	}
	for _, sp := range snap.Sprints {
		existing, _ := d.rawStore.GetSprint(sp.ID)
		if existing == nil {
			if _, err := d.rawStore.CreateSprint(sp); err != nil {
				return fmt.Errorf("create sprint %d: %w", sp.ID, err)
			}
		} else {
			if err := d.rawStore.UpdateSprint(sp); err != nil {
				return fmt.Errorf("update sprint %d: %w", sp.ID, err)
			}
		}
	}
	for _, t := range snap.Tasks {
		existing, _ := d.rawStore.GetTask(t.ID)
		if existing == nil {
			if err := d.rawStore.CreateTask(t); err != nil {
				return fmt.Errorf("create task %s: %w", t.ID, err)
			}
		} else {
			if err := d.rawStore.UpdateTask(t); err != nil {
				return fmt.Errorf("update task %s: %w", t.ID, err)
			}
		}
	}
	for _, pid := range projectIDs {
		if err := d.store.SetProjectSyncCursor(pid, snap.Cursor); err != nil {
			return fmt.Errorf("set sync cursor for project %s: %w", pid, err)
		}
		if err := d.store.SetSyncOrigin(pid, d.client.PrimaryURL()); err != nil {
			return fmt.Errorf("set sync origin for project %s: %w", pid, err)
		}
	}
	return nil
}

func (d *Daemon) Start(syncedProjects []string, startCursor int64) {
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	d.client.SetOnForbidden(func(pids []string) {
		for _, pid := range pids {
			_ = d.store.SetProjectSyncStatus(pid, "frozen")
		}
	})
	// Note: d.client.Start() is NOT called here — the caller starts the client heartbeat.
	go d.client.ConsumeStream(ctx, syncedProjects, startCursor, d.applyEvent)
}

func (d *Daemon) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
	d.client.Stop()
}

func (d *Daemon) State() SyncState { return d.client.State() }

// applyEvent writes an inbound event from the primary into the local store.
// The cursor always advances — even on a parse error — so a malformed event
// in the change log doesn't stall the stream permanently.
func (d *Daemon) applyEvent(ev ChangeEvent) {
	defer func() { _ = d.store.SetProjectSyncCursor(ev.ProjectID, ev.Cursor) }()
	switch ev.EventType {
	case EventTaskCreated, EventTaskUpdated:
		var t domain.Task
		if json.Unmarshal(ev.Payload, &t) != nil {
			return
		}
		existing, err := d.rawStore.GetTask(t.ID)
		if err != nil || existing == nil {
			_ = d.rawStore.CreateTask(&t)
		} else {
			_ = d.rawStore.UpdateTask(&t)
		}
	case EventTaskDeleted:
		var m map[string]string
		if json.Unmarshal(ev.Payload, &m) != nil {
			return
		}
		_ = d.rawStore.DeleteTask(m["id"])
	case EventSprintCreated, EventSprintUpdated:
		var sp domain.Sprint
		if json.Unmarshal(ev.Payload, &sp) != nil {
			return
		}
		existing, _ := d.rawStore.GetSprint(sp.ID)
		if existing == nil {
			_, _ = d.rawStore.CreateSprint(&sp)
		} else {
			_ = d.rawStore.UpdateSprint(&sp)
		}
	case EventUserMirrored:
		var u domain.User
		if json.Unmarshal(ev.Payload, &u) != nil {
			return
		}
		existing, _ := d.rawStore.GetUser(u.ID)
		if existing == nil {
			_ = d.rawStore.CreateUser(&u)
		}
	}
}
