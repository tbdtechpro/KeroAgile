package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/api"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// patchFixture is a server holding project TST with one ticket, TST-001, and
// a logged-in user "testuser".
type patchFixture struct {
	srv   *api.Server
	svc   *domain.Service
	token string
}

func newPatchFixture(t *testing.T) patchFixture {
	t.Helper()
	srv, svc, _ := newTestServer(t)
	require.NoError(t, svc.CreateProject("TST", "Test", ""))
	token := loginTestUser(t, srv, svc)
	_, err := svc.CreateTask("original title", "original description", "TST", domain.TaskCreateOpts{})
	require.NoError(t, err)
	return patchFixture{srv: srv, svc: svc, token: token}
}

// patch sends PATCH /api/tasks/TST-001 and returns the status and reply.
func (f patchFixture) patch(t *testing.T, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/tasks/TST-001", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+f.token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	f.srv.ServeHTTP(w, req)
	var reply map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &reply)
	return w.Code, reply
}

func (f patchFixture) ticket(t *testing.T) *domain.Task {
	t.Helper()
	task, err := f.svc.GetTask("TST-001")
	require.NoError(t, err)
	return task
}

// Reported: a save that changed the status applied only the status, dropped
// every other field in the request, and still answered 200.
func TestPatchAppliesAStatusChangeTogetherWithOtherFields(t *testing.T) {
	f := newPatchFixture(t)

	code, reply := f.patch(t, `{"status":"review","title":"renamed","priority":"high"}`)

	require.Equal(t, http.StatusOK, code)
	got := f.ticket(t)
	assert.Equal(t, domain.StatusReview, got.Status)
	assert.Equal(t, "renamed", got.Title)
	assert.Equal(t, domain.PriorityHigh, got.Priority)
	assert.Equal(t, "renamed", reply["title"], "the reply is the ticket as saved")
}

// Reported: assignee, points and sprint could be set but never emptied.
func TestPatchWithNullEmptiesAField(t *testing.T) {
	f := newPatchFixture(t)
	sprint, err := f.svc.CreateSprint("Sprint 1", "TST", nil, nil)
	require.NoError(t, err)
	user, five := "testuser", 5
	_, err = f.svc.PatchTask("TST-001", domain.TaskPatch{
		AssigneeID: domain.To(&user), Points: domain.To(&five),
		SprintID: domain.To(&sprint.ID), Labels: domain.To([]string{"a"}),
	})
	require.NoError(t, err)

	code, _ := f.patch(t, `{"assignee_id":null,"points":null,"sprint_id":null,"labels":null}`)

	require.Equal(t, http.StatusOK, code)
	got := f.ticket(t)
	assert.Nil(t, got.AssigneeID)
	assert.Nil(t, got.Points)
	assert.Nil(t, got.SprintID)
	assert.Empty(t, got.Labels)
	assert.Equal(t, "original title", got.Title, "fields not named are left alone")
}

func TestPatchWithAnEmptyAssigneeStringAlsoClearsIt(t *testing.T) {
	f := newPatchFixture(t)
	user := "testuser"
	_, err := f.svc.PatchTask("TST-001", domain.TaskPatch{AssigneeID: domain.To(&user)})
	require.NoError(t, err)

	code, _ := f.patch(t, `{"assignee_id":""}`)

	require.Equal(t, http.StatusOK, code)
	assert.Nil(t, f.ticket(t).AssigneeID)
}

func TestPatchSetsOptionalFields(t *testing.T) {
	f := newPatchFixture(t)
	sprint, err := f.svc.CreateSprint("Sprint 1", "TST", nil, nil)
	require.NoError(t, err)

	code, _ := f.patch(t, `{"assignee_id":"testuser","points":8,"labels":["x","y"],"sprint_id":`+jsonInt(sprint.ID)+`,"description":""}`)

	require.Equal(t, http.StatusOK, code)
	got := f.ticket(t)
	assert.Equal(t, "testuser", *got.AssigneeID)
	assert.Equal(t, 8, *got.Points)
	assert.Equal(t, []string{"x", "y"}, got.Labels)
	assert.Equal(t, sprint.ID, *got.SprintID)
	assert.Empty(t, got.Description, "an empty string empties the description")
}

// A client can send back the updated_at it read. If the ticket has changed
// since, the save is refused instead of being applied on top.
func TestPatchWithAStaleExpectedUpdatedAtIsAConflict(t *testing.T) {
	f := newPatchFixture(t)
	read, err := json.Marshal(f.ticket(t).UpdatedAt)
	require.NoError(t, err)
	_, err = f.svc.MoveTask("TST-001", domain.StatusTodo) // someone else changes it
	require.NoError(t, err)

	code, reply := f.patch(t, `{"title":"mine","expected_updated_at":`+string(read)+`}`)

	assert.Equal(t, http.StatusConflict, code)
	assert.Contains(t, reply["error"], "changed")
	assert.Equal(t, "original title", f.ticket(t).Title, "nothing was applied")
}

func TestPatchWithACurrentExpectedUpdatedAtIsApplied(t *testing.T) {
	f := newPatchFixture(t)
	_, err := f.svc.MoveTask("TST-001", domain.StatusTodo)
	require.NoError(t, err)
	read, err := json.Marshal(f.ticket(t).UpdatedAt)
	require.NoError(t, err)

	code, _ := f.patch(t, `{"title":"mine","expected_updated_at":`+string(read)+`}`)

	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "mine", f.ticket(t).Title)
}

func TestPatchAnswersBadRequestsWithTheRightStatus(t *testing.T) {
	cases := []struct {
		name, body string
		want       int
	}{
		{"unknown status", `{"status":"doing"}`, http.StatusBadRequest},
		{"unknown priority", `{"priority":"urgent"}`, http.StatusBadRequest},
		{"blank title", `{"title":"  "}`, http.StatusBadRequest},
		{"points not a number", `{"points":"five"}`, http.StatusBadRequest},
		{"labels not a list", `{"labels":"a,b"}`, http.StatusBadRequest},
		{"timestamp not a time", `{"expected_updated_at":"yesterday"}`, http.StatusBadRequest},
		{"not JSON", `{`, http.StatusBadRequest},
		{"unknown assignee", `{"assignee_id":"nobody"}`, http.StatusNotFound},
		{"unknown sprint", `{"sprint_id":999}`, http.StatusNotFound},
	}
	for _, c := range cases {
		f := newPatchFixture(t)

		code, _ := f.patch(t, c.body)

		assert.Equal(t, c.want, code, c.name)
		assert.Equal(t, "original title", f.ticket(t).Title, c.name)
	}
}

func jsonInt(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
