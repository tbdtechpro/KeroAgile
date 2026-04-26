package syncsrv

import (
	"context"
	"encoding/json"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// Daemon orchestrates a secondary's background sync work.
type Daemon struct {
	client   *Client
	store    SecondaryStore // for SetProjectSyncCursor, SetProjectSyncStatus
	rawStore domain.Store   // for task/sprint/user upserts
	cancel   context.CancelFunc
}

// NewDaemon creates a Daemon. st must implement SecondaryStore; rawStore is for entity upserts.
// Passing the same *store.Store for both is fine — it implements all needed interfaces.
func NewDaemon(cfg ClientConfig, st SecondaryStore, rawStore domain.Store) *Daemon {
	return &Daemon{
		client:   NewClient(cfg, st),
		store:    st,
		rawStore: rawStore,
	}
}

func (d *Daemon) Start(syncedProjects []string, startCursor int64) {
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	d.client.Start()
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
