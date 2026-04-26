package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type SyncConfig struct {
	Mode              string `toml:"mode"`
	PrimaryURL        string `toml:"primary_url"`
	SecondaryID       string `toml:"secondary_id"`
	APIToken          string `toml:"api_token"`
	HeartbeatInterval string `toml:"heartbeat_interval"`
	OfflineThreshold  int    `toml:"offline_threshold"`
}

type Config struct {
	DefaultProject  string `toml:"default_project"`
	DefaultAssignee string `toml:"default_assignee"`
	// API server settings (used by 'KeroAgile serve')
	APISecret   string     `toml:"api_secret"`   // JWT signing key; auto-generated if empty
	RemoteURL   string     `toml:"remote_url"`   // optional: connect to a remote KeroAgile server
	RemoteToken string     `toml:"remote_token"` // bearer token for remote_url
	Sync        SyncConfig `toml:"sync"`
}

func DefaultPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "keroagile", "config.toml")
}

func DBPath() string {
	if dir := os.Getenv("KEROAGILE_DATA_DIR"); dir != "" {
		return filepath.Join(dir, "keroagile.db")
	}
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "keroagile", "keroagile.db")
}

func Load() (*Config, error) {
	path := DefaultPath()
	cfg := &Config{}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}
	_, err := toml.DecodeFile(path, cfg)
	return cfg, err
}

func Save(cfg *Config) error {
	path := DefaultPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}
