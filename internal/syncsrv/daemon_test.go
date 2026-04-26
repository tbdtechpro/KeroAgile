package syncsrv_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tbdtechpro/KeroAgile/internal/syncsrv"
)

func TestHeartbeatStateTransitions(t *testing.T) {
	var serverUp bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serverUp {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()

	cfg := syncsrv.ClientConfig{
		PrimaryURL:       server.URL,
		APIToken:         "test-token",
		HeartbeatEvery:   50 * time.Millisecond,
		OfflineThreshold: 3,
	}
	client := syncsrv.NewClient(cfg, nil) // nil store — just testing state machine

	serverUp = true
	client.Start()
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, syncsrv.StateOnline, client.State())

	serverUp = false
	time.Sleep(500 * time.Millisecond) // 10+ heartbeat intervals
	assert.Equal(t, syncsrv.StateOffline, client.State())

	serverUp = true
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, syncsrv.StateOnline, client.State())

	client.Stop()
}
