package syncsrv

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ClientConfig configures a secondary's connection to its primary.
type ClientConfig struct {
	PrimaryURL       string
	APIToken         string
	SecondaryID      string
	HeartbeatEvery   time.Duration
	OfflineThreshold int
}

func (c ClientConfig) heartbeatInterval() time.Duration {
	if c.HeartbeatEvery == 0 {
		return 15 * time.Second
	}
	return c.HeartbeatEvery
}

func (c ClientConfig) threshold() int {
	if c.OfflineThreshold == 0 {
		return 3
	}
	return c.OfflineThreshold
}

// Client manages the secondary's connection to the primary: heartbeat + SSE stream.
type Client struct {
	cfg         ClientConfig
	store       SecondaryStore
	state       atomic.Value // stores SyncState
	missed      int
	mu          sync.Mutex
	cancel      context.CancelFunc
	hc          *http.Client
	StateChange func(SyncState) // optional callback on state change
}

func NewClient(cfg ClientConfig, store SecondaryStore) *Client {
	c := &Client{cfg: cfg, store: store, hc: &http.Client{Timeout: 10 * time.Second}}
	c.state.Store(StateOnline)
	return c
}

func (c *Client) State() SyncState {
	return c.state.Load().(SyncState)
}

func (c *Client) setState(s SyncState) {
	old := c.State()
	if old == s {
		return
	}
	c.state.Store(s)
	if c.StateChange != nil {
		c.StateChange(s)
	}
}

func (c *Client) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	go c.heartbeatLoop(ctx)
}

func (c *Client) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
}

func (c *Client) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(c.cfg.heartbeatInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.doHeartbeat()
		}
	}
}

func (c *Client) doHeartbeat() {
	req, err := http.NewRequest("GET", c.cfg.PrimaryURL+"/api/sync/heartbeat", nil)
	if err != nil {
		c.countMiss()
		return
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	resp, err := c.hc.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		c.countMiss()
		if resp != nil {
			resp.Body.Close()
		}
		return
	}
	resp.Body.Close()
	c.mu.Lock()
	c.missed = 0
	c.mu.Unlock()
	c.setState(StateOnline)
}

func (c *Client) countMiss() {
	c.mu.Lock()
	c.missed++
	missed := c.missed
	c.mu.Unlock()
	if missed == 1 {
		c.setState(StateReconnecting)
	} else if missed >= c.cfg.threshold() {
		c.setState(StateOffline)
	}
}

// Proxy forwards a mutation to the primary, replacing auth with the sync token.
func (c *Client) Proxy(w http.ResponseWriter, r *http.Request, targetPath string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusInternalServerError)
		return
	}
	url := c.cfg.PrimaryURL + targetPath
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, url, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "proxy error", http.StatusBadGateway)
		return
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	req.Header.Set("X-Sync-Origin", c.cfg.SecondaryID)
	req.Header.Set("Content-Type", r.Header.Get("Content-Type"))

	resp, err := c.hc.Do(req)
	if err != nil {
		http.Error(w, "primary unreachable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// ConsumeStream connects to the primary's SSE stream and calls apply for each inbound change.
// Reconnects automatically until ctx is cancelled.
func (c *Client) ConsumeStream(ctx context.Context, projectIDs []string, cursor int64, apply func(ChangeEvent)) {
	ids := strings.Join(projectIDs, "&project_ids=")
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		url := fmt.Sprintf("%s/api/sync/stream?project_ids=%s&since=%d", c.cfg.PrimaryURL, ids, cursor)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
		resp, err := c.hc.Do(req)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		scanner := bufio.NewScanner(resp.Body)
		var dataLine string
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				dataLine = strings.TrimPrefix(line, "data: ")
			} else if line == "" && dataLine != "" {
				var ev ChangeEvent
				if json.Unmarshal([]byte(dataLine), &ev) == nil && ev.Cursor > 0 {
					apply(ev)
					cursor = ev.Cursor
				}
				dataLine = ""
			}
		}
		resp.Body.Close()
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}
