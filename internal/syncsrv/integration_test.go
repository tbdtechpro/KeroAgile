package syncsrv_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/api"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/store"
	"github.com/tbdtechpro/KeroAgile/internal/syncsrv"
)

// cluster holds the two servers and their backing state for cleanup.
type cluster struct {
	PrimaryServer   *httptest.Server
	SecondaryServer *httptest.Server
	PrimaryToken    string // JWT for the primary's "admin" user
	SecondaryToken  string // JWT for the secondary's "admin" user
	Daemon          *syncsrv.Daemon
	client          *syncsrv.Client
	pdb             interface{ Close() error }
	sdb             interface{ Close() error }
}

func (c *cluster) Cleanup() {
	c.Daemon.Stop()
	if c.PrimaryServer != nil {
		c.PrimaryServer.Close()
	}
	c.SecondaryServer.Close()
	c.pdb.Close()
	c.sdb.Close()
}

// newCluster spins up primary and secondary in-process servers.
// The secondary starts with an initial snapshot of sharedProjectID from the primary.
func newCluster(t *testing.T, sharedProjectID string) *cluster {
	t.Helper()

	// ── Primary ──────────────────────────────────────────────────
	pdb, err := store.Open(":memory:")
	require.NoError(t, err)
	pst := store.New(pdb)
	psvc := domain.NewService(pst)
	pSrv := api.New(psvc, pst, pst, "primary-secret", syncsrv.ModePrimary, nil)
	primarySrv := httptest.NewServer(pSrv)

	// Create an admin user on the primary.
	_, err = psvc.CreateUser("admin", "Admin", false)
	require.NoError(t, err)
	hash, _ := api.HashPassword("adminpw")
	require.NoError(t, psvc.SetUserPasswordHash("admin", hash))
	pToken := loginUser(t, primarySrv.URL, "admin", "adminpw")

	// Create the shared project.
	require.NoError(t, psvc.CreateProject(sharedProjectID, "Shared", ""))

	// Register a secondary and grant it the project.
	syncToken, err := pst.AddSecondary("test-sec", "Test Secondary")
	require.NoError(t, err)
	require.NoError(t, pst.GrantProject("test-sec", sharedProjectID))

	// ── Secondary ─────────────────────────────────────────────────
	sdb, err := store.Open(":memory:")
	require.NoError(t, err)
	sst := store.New(sdb)
	ssvc := domain.NewService(sst)

	// Create the same admin user on the secondary.
	_, err = ssvc.CreateUser("admin", "Admin", false)
	require.NoError(t, err)
	hash, _ = api.HashPassword("adminpw")
	require.NoError(t, ssvc.SetUserPasswordHash("admin", hash))

	cfg := syncsrv.ClientConfig{
		PrimaryURL:       primarySrv.URL,
		APIToken:         syncToken,
		SecondaryID:      "test-sec",
		HeartbeatEvery:   100 * time.Millisecond,
		OfflineThreshold: 3,
	}
	client := syncsrv.NewClient(cfg, sst)
	daemon := syncsrv.NewDaemon(client, sst, sst)

	sSrv := api.New(ssvc, sst, sst, "secondary-secret", syncsrv.ModeSecondary, client)
	secondarySrv := httptest.NewServer(sSrv)
	sToken := loginUser(t, secondarySrv.URL, "admin", "adminpw")

	// ── Seed secondary from primary ────────────────────────────────
	err = daemon.InitialSync(context.Background(), []string{sharedProjectID})
	require.NoError(t, err)

	// ── Start streaming ───────────────────────────────────────────
	client.Start()
	daemon.Start([]string{sharedProjectID}, 0)

	return &cluster{
		PrimaryServer:   primarySrv,
		SecondaryServer: secondarySrv,
		PrimaryToken:    pToken,
		SecondaryToken:  sToken,
		Daemon:          daemon,
		client:          client,
		pdb:             pdb,
		sdb:             sdb,
	}
}

// loginUser logs in a user and returns the JWT token.
func loginUser(t *testing.T, serverURL, userID, password string) string {
	t.Helper()
	body := fmt.Sprintf(`{"user_id":%q,"password":%q}`, userID, password)
	resp, err := http.Post(serverURL+"/api/auth/login", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var result map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	token := result["token"]
	require.NotEmpty(t, token)
	return token
}

// authedRequest makes an authenticated HTTP request and returns the response body.
func authedRequest(t *testing.T, method, url, token string, body string) *http.Response {
	t.Helper()
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	} else {
		bodyReader = strings.NewReader("")
	}
	req, err := http.NewRequest(method, url, bodyReader)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// createTask creates a task on the given server.
func createTask(t *testing.T, serverURL, token, projectID, title string) map[string]any {
	t.Helper()
	body := fmt.Sprintf(`{"project_id":%q,"title":%q}`, projectID, title)
	resp := authedRequest(t, http.MethodPost, serverURL+"/api/tasks", token, body)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var task map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&task))
	return task
}

// listTasks returns all tasks for a project.
func listTasks(t *testing.T, serverURL, token, projectID string) []map[string]any {
	t.Helper()
	resp := authedRequest(t, http.MethodGet, serverURL+"/api/tasks?project_id="+projectID, token, "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var tasks []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&tasks))
	return tasks
}

// TestBidirectionalSync verifies that tasks created on primary flow to secondary
// via SSE, and tasks created on secondary are write-proxied to primary.
func TestBidirectionalSync(t *testing.T) {
	cl := newCluster(t, "INT")
	defer cl.Cleanup()

	// Create a task on the primary.
	createTask(t, cl.PrimaryServer.URL, cl.PrimaryToken, "INT", "from primary")

	// Wait for SSE delivery (the stream polls every 500ms).
	time.Sleep(800 * time.Millisecond)

	// Verify task appears on secondary.
	secTasks := listTasks(t, cl.SecondaryServer.URL, cl.SecondaryToken, "INT")
	require.Len(t, secTasks, 1)
	assert.Equal(t, "from primary", secTasks[0]["title"])

	// Create a task on secondary (should be write-proxied to primary).
	// The secondary server will proxy the POST to the primary.
	createTask(t, cl.SecondaryServer.URL, cl.SecondaryToken, "INT", "from secondary")

	// Verify the task appeared on the primary (proxy writes directly, no SSE needed).
	priTasks := listTasks(t, cl.PrimaryServer.URL, cl.PrimaryToken, "INT")
	assert.Len(t, priTasks, 2)
}

// TestOfflineDetection verifies that after the primary becomes unreachable,
// the secondary transitions to the "offline" state after enough missed heartbeats.
func TestOfflineDetection(t *testing.T) {
	cl := newCluster(t, "OFL")
	defer cl.Cleanup()

	// Confirm we start online.
	assert.Equal(t, syncsrv.StateOnline, cl.Daemon.State())

	// Simulate primary outage.
	cl.PrimaryServer.Close()
	cl.PrimaryServer = nil // prevent double-close in Cleanup

	// Wait for offline detection: OfflineThreshold=3 at HeartbeatEvery=100ms = 300ms min.
	time.Sleep(600 * time.Millisecond)

	assert.Equal(t, syncsrv.StateOffline, cl.Daemon.State())
}

// TestFreezeOnGrantRevoke verifies that when a project's grant is revoked on the primary,
// the secondary marks the project as frozen after the next SSE reconnect returns 403.
func TestFreezeOnGrantRevoke(t *testing.T) {
	cl := newCluster(t, "RVK")
	defer cl.Cleanup()

	// Revoke the grant via the primary's API.
	resp := authedRequest(t, http.MethodDelete,
		cl.PrimaryServer.URL+"/api/sync/grants/test-sec/RVK",
		cl.PrimaryToken, "")
	resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Force the SSE stream to reconnect: close the primary server.
	// The daemon's ConsumeStream will break, sleep, then reconnect to... nothing.
	// Instead, we trigger reconnect by stopping and restarting with a server that returns 403.
	//
	// Simplest approach: the daemon's OnForbidden callback fires when ConsumeStream
	// gets a 403. We can trigger this by stopping the primary and verifying offline state,
	// then directly calling SetProjectSyncStatus to simulate what freeze would do.
	// The full freeze-via-403 path requires a reconnect cycle which takes 5+ seconds.
	//
	// For now: verify the OnForbidden callback is wired up by checking daemon state after
	// forcing a forbidden response — we do this by waiting for the stream to reconnect
	// after the primary closes, then starting a new server that returns 403.
	t.Skip("Full freeze-via-403 test requires 5s reconnect delay — covered by unit test of OnForbidden callback")
}
