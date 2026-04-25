package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	DefaultProject  string `toml:"default_project"`
	DefaultAssignee string `toml:"default_assignee"`
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
