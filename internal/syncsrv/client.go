package syncsrv

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
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

// SnapshotResult holds the snapshot response from the primary.
type SnapshotResult struct {
	Projects []*domain.Project `json:"projects"`
	Tasks    []*domain.Task    `json:"tasks"`
	Sprints  []*domain.Sprint  `json:"sprints"`
	Users    []*domain.User    `json:"users"`
	Cursor   int64             `json:"cursor"`
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
	onForbidden func([]string) // called when primary returns 403 for the stream; guarded by mu
}

func NewClient(cfg ClientConfig, store SecondaryStore) *Client {
	c := &Client{cfg: cfg, store: store, hc: &http.Client{Timeout: 10 * time.Second}}
	c.state.Store(StateOnline)
	return c
}

// SetOnForbidden sets the callback invoked when the primary returns 403 on the stream.
func (c *Client) SetOnForbidden(fn func([]string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onForbidden = fn
}

func (c *Client) getOnForbidden() func([]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.onForbidden
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

// PrimaryURL returns the configured primary URL.
func (c *Client) PrimaryURL() string { return c.cfg.PrimaryURL }

// FetchSnapshot calls the primary's /api/sync/snapshot endpoint and returns parsed data.
func (c *Client) FetchSnapshot(ctx context.Context, projectIDs []string) (*SnapshotResult, error) {
	q := url.Values{}
	for _, id := range projectIDs {
		q.Add("project_ids", id)
	}
	u := c.cfg.PrimaryURL + "/api/sync/snapshot?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("snapshot: primary returned %s", resp.Status)
	}
	var result SnapshotResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("snapshot: decode: %w", err)
	}
	return &result, nil
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
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		q := url.Values{}
		for _, id := range projectIDs {
			q.Add("project_ids", id)
		}
		streamURL := fmt.Sprintf("%s/api/sync/stream?since=%d&%s", c.cfg.PrimaryURL, cursor, q.Encode())
		req, err := http.NewRequestWithContext(ctx, "GET", streamURL, nil)
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
		if resp.StatusCode == http.StatusForbidden {
			resp.Body.Close()
			if fn := c.getOnForbidden(); fn != nil {
				fn(projectIDs)
			}
			return // don't retry — frozen, needs manual intervention
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
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
