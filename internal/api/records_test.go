package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// do sends an authenticated request and returns the status and raw reply.
func (f patchFixture) do(t *testing.T, method, path, body string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+f.token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	f.srv.ServeHTTP(w, req)
	return w.Code, w.Body.Bytes()
}

func TestAddNote_UsesTokenUser(t *testing.T) {
	f := newPatchFixture(t)

	code, body := f.do(t, http.MethodPost, "/api/tasks/tst-001/notes", `{"body":"  from the web  "}`)
	require.Equal(t, http.StatusCreated, code, string(body))
	var n map[string]any
	require.NoError(t, json.Unmarshal(body, &n))
	assert.Equal(t, "testuser", n["author_id"])
	assert.Equal(t, "from the web", n["body"])

	code, body = f.do(t, http.MethodGet, "/api/tasks/TST-001", "")
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(body), "from the web")

	code, _ = f.do(t, http.MethodPost, "/api/tasks/TST-001/notes", `{"body":"   "}`)
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = f.do(t, http.MethodPost, "/api/tasks/TST-404/notes", `{"body":"x"}`)
	assert.Equal(t, http.StatusNotFound, code)
}
