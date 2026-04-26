package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/api"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/store"
	"github.com/tbdtechpro/KeroAgile/internal/syncsrv"
)

func newTestServer(t *testing.T) (*api.Server, *domain.Service, *store.Store) {
	t.Helper()
	db, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	st := store.New(db)
	svc := domain.NewService(st)
	return api.New(svc, st, st, "test-secret", syncsrv.ModeStandalone, nil), svc, st
}

func TestLoginInvalidCredentials(t *testing.T) {
	srv, _, _ := newTestServer(t)
	body := `{"user_id":"alice","password":"wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLoginAndAccessProtected(t *testing.T) {
	srv, svc, _ := newTestServer(t)

	// Create a user and set password
	_, err := svc.CreateUser("alice", "Alice", false)
	require.NoError(t, err)
	hash, err := api.HashPassword("secret123")
	require.NoError(t, err)
	require.NoError(t, svc.SetUserPasswordHash("alice", hash))

	// Login
	loginBody, _ := json.Marshal(map[string]string{"user_id": "alice", "password": "secret123"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(loginBody))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	token := resp["token"]
	require.NotEmpty(t, token)

	// Access protected endpoint
	req2 := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)
}

func TestProtectedWithoutToken(t *testing.T) {
	srv, _, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestTaskCRUD(t *testing.T) {
	srv, svc, _ := newTestServer(t)

	require.NoError(t, svc.CreateProject("TST", "Test", ""))

	_, err := svc.CreateUser("bob", "Bob", false)
	require.NoError(t, err)
	hash, _ := api.HashPassword("pw")
	require.NoError(t, svc.SetUserPasswordHash("bob", hash))

	// Login
	loginBody, _ := json.Marshal(map[string]string{"user_id": "bob", "password": "pw"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(loginBody))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var lr map[string]string
	json.NewDecoder(w.Body).Decode(&lr)
	token := lr["token"]

	authed := func(method, path string, body []byte) *httptest.ResponseRecorder {
		var buf *bytes.Buffer
		if body != nil {
			buf = bytes.NewBuffer(body)
		} else {
			buf = &bytes.Buffer{}
		}
		r := httptest.NewRequest(method, path, buf)
		r.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, r)
		return rr
	}

	// Create task
	createBody, _ := json.Marshal(map[string]any{"title": "Fix bug", "project_id": "TST"})
	rr := authed(http.MethodPost, "/api/tasks", createBody)
	require.Equal(t, http.StatusCreated, rr.Code)

	var task map[string]any
	json.NewDecoder(rr.Body).Decode(&task)
	taskID := task["id"].(string)

	// Get task
	rr = authed(http.MethodGet, "/api/tasks/"+taskID, nil)
	require.Equal(t, http.StatusOK, rr.Code)

	// Move task
	moveBody, _ := json.Marshal(map[string]string{"status": "in_progress"})
	rr = authed(http.MethodPatch, "/api/tasks/"+taskID, moveBody)
	require.Equal(t, http.StatusOK, rr.Code)

	// Delete task
	rr = authed(http.MethodDelete, "/api/tasks/"+taskID, nil)
	require.Equal(t, http.StatusNoContent, rr.Code)
}

// loginTestUser creates a test user, sets a password, logs in, and returns the JWT token.
func loginTestUser(t *testing.T, srv *api.Server, svc *domain.Service) string {
	t.Helper()
	_, err := svc.CreateUser("testuser", "Test User", false)
	require.NoError(t, err)
	hash, err := api.HashPassword("testpw")
	require.NoError(t, err)
	require.NoError(t, svc.SetUserPasswordHash("testuser", hash))

	loginBody, _ := json.Marshal(map[string]string{"user_id": "testuser", "password": "testpw"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(loginBody))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	token := resp["token"]
	require.NotEmpty(t, token)
	return token
}

func TestMutationWritesChangeLog(t *testing.T) {
	srv, svc, st := newTestServer(t)
	token := loginTestUser(t, srv, svc)

	require.NoError(t, svc.CreateProject("TL", "TestLog", ""))

	body := `{"project_id":"TL","title":"hello"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	events, err := st.ReadChanges("TL", 0)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "task.created", events[0].EventType)
}

func TestSyncedProjectWriteIsProxied(t *testing.T) {
	var received []string
	fakePrimary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = append(received, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": "TL-002"})
	}))
	defer fakePrimary.Close()

	_, svc, st := newTestServer(t)

	// Build a secondary-mode server with a fake-primary client.
	cfg := syncsrv.ClientConfig{
		PrimaryURL: fakePrimary.URL,
		APIToken:   "tok",
	}
	client := syncsrv.NewClient(cfg, st)
	srv := api.New(svc, st, st, "test-secret", syncsrv.ModeSecondary, client)

	require.NoError(t, svc.CreateProject("TL", "TestLog", ""))
	require.NoError(t, st.SetSyncOrigin("TL", fakePrimary.URL))

	token := loginTestUser(t, srv, svc)
	body := `{"project_id":"TL","title":"proxied"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, received, "POST /api/tasks")

	// Proxy must not write the task to the local DB.
	tasks, err := st.ReadChanges("TL", 0)
	require.NoError(t, err)
	assert.Empty(t, tasks, "proxy must not write task to local change_log")
}

func TestSyncHeartbeatRequiresAuth(t *testing.T) {
	srv, _, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/sync/heartbeat", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAddAndListSecondaries(t *testing.T) {
	srv, svc, _ := newTestServer(t)

	// Create an admin user and obtain a JWT.
	_, err := svc.CreateUser("admin", "Admin", false)
	require.NoError(t, err)
	hash, err := api.HashPassword("adminpw")
	require.NoError(t, err)
	require.NoError(t, svc.SetUserPasswordHash("admin", hash))

	loginBody, _ := json.Marshal(map[string]string{"user_id": "admin", "password": "adminpw"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(loginBody))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var lr map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&lr))
	jwtToken := lr["token"]
	require.NotEmpty(t, jwtToken)

	authed := func(method, path string, body []byte) *httptest.ResponseRecorder {
		var buf *bytes.Buffer
		if body != nil {
			buf = bytes.NewBuffer(body)
		} else {
			buf = &bytes.Buffer{}
		}
		r := httptest.NewRequest(method, path, buf)
		r.Header.Set("Authorization", "Bearer "+jwtToken)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, r)
		return rr
	}

	// POST /api/sync/secondaries — register a new secondary.
	addBody, _ := json.Marshal(map[string]string{"id": "office-node", "display_name": "Office Node"})
	rr := authed(http.MethodPost, "/api/sync/secondaries", addBody)
	require.Equal(t, http.StatusCreated, rr.Code)

	var addResp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&addResp))
	syncToken := addResp["token"]
	require.NotEmpty(t, syncToken)

	// GET /api/sync/secondaries — list secondaries.
	rr = authed(http.MethodGet, "/api/sync/secondaries", nil)
	require.Equal(t, http.StatusOK, rr.Code)

	var listResp []map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&listResp))
	require.Len(t, listResp, 1)
	require.Equal(t, "office-node", listResp[0]["id"])

	// GET /api/sync/heartbeat with the sync token — should return 200.
	heartbeatReq := httptest.NewRequest(http.MethodGet, "/api/sync/heartbeat", nil)
	heartbeatReq.Header.Set("Authorization", "Bearer "+syncToken)
	hw := httptest.NewRecorder()
	srv.ServeHTTP(hw, heartbeatReq)
	require.Equal(t, http.StatusOK, hw.Code)

	var hbResp map[string]string
	require.NoError(t, json.NewDecoder(hw.Body).Decode(&hbResp))
	require.Equal(t, "true", hbResp["ok"])
	require.NotEmpty(t, hbResp["ts"])
}
